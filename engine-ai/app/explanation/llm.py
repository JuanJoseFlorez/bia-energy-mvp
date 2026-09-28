"""LLM drafts through LiteLLM. The only module that imports litellm."""

import json
import logging
import random
import time
from collections.abc import Callable
from concurrent.futures import Executor, ThreadPoolExecutor, wait

import litellm

from app.config import Settings
from app.explanation.review import (
    ACTION_DETAIL_MAX,
    EXPLANATION_MAX,
    REASON_MAX,
    Draft,
    Drafter,
    DraftRequest,
    validate,
)

logger = logging.getLogger(__name__)

# LiteLLM: warnings and errors only, through the root JSON handler (no own stderr handler).
litellm.suppress_debug_info = True
for _name in ("LiteLLM", "LiteLLM Router", "LiteLLM Proxy"):
    logging.getLogger(_name).handlers = []
    logging.getLogger(_name).setLevel(logging.WARNING)
logging.getLogger("httpx").setLevel(logging.WARNING)  # no per-request lines

SYSTEM_PROMPT = f"""Eres un analista de energía que explica hallazgos ya calculados por un \
motor determinista. El tipo, la severidad y la confianza ya están decididos: no los cambies \
ni los discutas.

Reglas:
- Escribe en español, en tono profesional, claro y concreto.
- Usa solo cifras, fechas y horas que aparezcan en HECHOS o EVENTOS, copiadas exactamente \
como están escritas (coma decimal, punto de miles, "%" y fechas como "12-sep 14:00").
- No agregues años, conteos, duraciones, diferencias ni cálculos propios. Si necesitas \
contar algo, hazlo con palabras.
- Las descripciones de EVENTOS están en inglés: úsalas como contexto, pero escribe en español.
- "action_detail" es un paso concreto coherente con la acción base; no repitas la acción \
base ni la contradigas.
- Responde solo con un objeto JSON con exactamente estas claves de texto:
  "reason": una sola línea de máximo {REASON_MAX} caracteres con el hallazgo principal.
  "explanation": un párrafo de máximo {EXPLANATION_MAX} caracteres que explique el \
hallazgo citando las cifras clave.
  "action_detail": una frase de máximo {ACTION_DETAIL_MAX} caracteres."""

Completion = Callable[..., object]
Sleep = Callable[[float], None]

MAX_TRANSIENT_RETRIES = 2  # provider overloaded / 5xx / 429 / connection / per-call timeout
MAX_INVALID_RETRIES = 1  # output that fails validation (JSON, schema, length, grounding)
BACKOFF_SECONDS = 1.0  # ~1 s before the first transient retry, ~2 s before the second
MIN_ATTEMPT_SECONDS = 2.0  # a retry starts only if at least this much time is left for it
RETRYABLE_STATUS = frozenset({408, 429, 500, 502, 503, 504})


def create_llm_pool(settings: Settings) -> ThreadPoolExecutor | None:
    """Process-wide pool that caps simultaneous LLM calls; None when the LLM is disabled."""
    if not settings.llm_enabled:
        return None
    return ThreadPoolExecutor(max_workers=settings.llm_max_concurrency, thread_name_prefix="llm")


def messages(request: DraftRequest) -> list[dict]:
    return [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": request.prompt},
    ]


def retryable(exc: Exception) -> bool:
    """Transient provider or network problem; client errors (400, 401, 404...) are final."""
    if isinstance(exc, litellm.APIConnectionError):
        return True
    return getattr(exc, "status_code", None) in RETRYABLE_STATUS  # litellm.Timeout is 408


def retry_after(exc: Exception) -> float | None:
    """Seconds from a Retry-After header, when the provider sent one."""
    response = getattr(exc, "response", None)
    for headers in (getattr(exc, "headers", None), getattr(response, "headers", None)):
        value = headers.get("retry-after") if headers else None
        if value is not None:
            try:
                return max(0.0, float(value))
            except ValueError:
                return None
    return None


def _parse(content: object, prompt: str) -> tuple[dict | None, str | None]:
    """The JSON object and its first problem (None when the draft passes review)."""
    try:
        data = json.loads(content)
    except (TypeError, ValueError):
        return None, "invalid_json"
    if not isinstance(data, dict):
        return None, "invalid_json"
    return data, validate(data, prompt)[1]


def draft_one(
    request: DraftRequest,
    model: str,
    api_key: str,
    deadline: float,
    completion: Completion,
    sleep: Sleep = time.sleep,
) -> Draft:
    """Draft for one anomaly with bounded retries, all before `deadline` (time.monotonic).

    Never raises: failures become a Draft with a failure reason. A draft whose last
    attempt still fails validation is returned as is, so review.explain rejects it.
    """
    started = time.monotonic()
    attempt = transient = invalid = 0
    context = {"meter_id": request.meter_id, "analysis_id": request.analysis_id}

    def result(**fields) -> Draft:
        elapsed = round((time.monotonic() - started) * 1000, 1)
        return Draft(duration_ms=elapsed, attempts=attempt, **fields)

    def retry(reason: str, pause: float) -> bool:
        """Sleep the full pause and allow a further attempt; False when it would not fit."""
        room = deadline - time.monotonic() - MIN_ATTEMPT_SECONDS
        if room <= 0 or pause > room:
            return False
        logger.info("llm_retry", extra={**context, "attempt": attempt + 1, "reason": reason})
        sleep(pause)
        return True

    logger.debug("llm prompt", extra={**context, "prompt": request.prompt})
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            return result(failure="timeout")
        attempt += 1
        try:
            response = completion(
                model=model,
                api_key=api_key,
                messages=messages(request),
                response_format={"type": "json_object"},
                timeout=remaining,
            )
            content = response.choices[0].message.content
        except Exception as exc:  # provider, network or response-shape errors
            # Only the class name: provider messages may echo request details.
            error = type(exc).__name__
            logger.warning("llm call failed", extra={**context, "attempt": attempt, "error": error})
            final = "timeout" if isinstance(exc, litellm.Timeout) else "error"
            if not retryable(exc) or transient == MAX_TRANSIENT_RETRIES:
                return result(failure=final)
            transient += 1
            backoff = BACKOFF_SECONDS * 2 ** (transient - 1) * random.uniform(0.8, 1.2)
            pause = retry_after(exc)
            if not retry(error, backoff if pause is None else pause):
                return result(failure="timeout")
            continue
        data, problem = _parse(content, request.prompt)
        if problem is None or invalid == MAX_INVALID_RETRIES:
            return result(data=data) if data is not None else result(failure=problem)
        invalid += 1
        if not retry(problem, 0.0):
            return result(failure="timeout")


def make_drafter(
    pool: Executor,
    model: str,
    api_key: str,
    timeout: float,
    completion: Completion = litellm.completion,
    sleep: Sleep = time.sleep,
) -> Drafter:
    """Drafter that sends every request to the shared pool and waits at most `timeout`."""

    def drafter(requests: list[DraftRequest]) -> dict[str, Draft]:
        deadline = time.monotonic() + timeout  # queue wait counts
        futures = {}
        for request in requests:
            try:
                futures[request.meter_id] = pool.submit(
                    draft_one, request, model, api_key, deadline, completion, sleep
                )
            except RuntimeError:  # pool already shut down
                logger.warning("llm pool unavailable", extra={"meter_id": request.meter_id})
        done, pending = wait(futures.values(), timeout=timeout)
        for future in pending:
            future.cancel()  # not started yet: frees the queue; running calls end by the deadline
        drafts = {}
        for request in requests:
            future = futures.get(request.meter_id)
            if future is None:
                drafts[request.meter_id] = Draft(failure="error")
            elif future in done:
                drafts[request.meter_id] = future.result()
            else:
                drafts[request.meter_id] = Draft(failure="timeout", duration_ms=timeout * 1000)
        return drafts

    return drafter

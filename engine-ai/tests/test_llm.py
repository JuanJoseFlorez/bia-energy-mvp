import json
import logging
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime
from types import SimpleNamespace

import litellm
import pytest

from app.config import load_settings
from app.explanation.facts import build_facts, render_prompt
from app.explanation.llm import SYSTEM_PROMPT, create_llm_pool, make_drafter, retryable
from app.explanation.review import DraftRequest, explain, validate
from app.explanation.templates import ACTION_BASE, template_texts
from tests.support import make_anomaly, make_events

GOOD = {"reason": "r", "explanation": "e", "action_detail": "a"}
BASE_ENV = {"DB_HOST": "db", "DB_USER": "u", "DB_PASSWORD": "p", "DB_NAME": "n"}


def response(content: str):
    return SimpleNamespace(choices=[SimpleNamespace(message=SimpleNamespace(content=content))])


def answering(content: str):
    def completion(**kwargs):
        return response(content)

    return completion


def requests(n: int, prefix: str = "X") -> list[DraftRequest]:
    return [DraftRequest(f"{prefix}-{i}", f"prompt {prefix}-{i}") for i in range(n)]


@pytest.fixture
def pool():
    executor = ThreadPoolExecutor(max_workers=2)
    yield executor
    executor.shutdown(wait=False, cancel_futures=True)


def test_prompt_carries_the_event_description_and_call_options(pool):
    events = make_events((3, "X-1", "2026-09-12 14:00", "UNKNOWN", "No operational event reported"))
    anomaly = make_anomaly(change_start=datetime(2026, 9, 12, 14), related_event_ids=[3])
    facts = build_facts(anomaly, events)
    request = DraftRequest("X-1", render_prompt(facts, ACTION_BASE["REAL_ANOMALY"]))
    calls = []

    def completion(**kwargs):
        calls.append(kwargs)
        return response(json.dumps(GOOD))

    drafts = make_drafter(pool, "gemini/test-model", "test-key", 7.5, completion)([request])

    assert drafts["X-1"].data == GOOD
    assert drafts["X-1"].failure is None
    [call] = calls
    assert call["model"] == "gemini/test-model"
    assert call["api_key"] == "test-key"
    assert "temperature" not in call  # provider default
    assert call["response_format"] == {"type": "json_object"}
    assert 7.0 < call["timeout"] <= 7.5  # time left before the analysis deadline
    system, user = call["messages"]
    assert system == {"role": "system", "content": SYSTEM_PROMPT}
    assert user["role"] == "user"
    assert "No operational event reported" in user["content"]
    texts = template_texts(facts)
    assert "No operational event reported" not in texts.reason + texts.explanation
    assert "No operational event reported" not in str(facts.lines())


def test_system_prompt_states_the_rules():
    for rule in ("JSON", "reason", "explanation", "action_detail", "HECHOS", "años", "español"):
        assert rule in SYSTEM_PROMPT


def test_exception_becomes_error_and_key_is_not_logged(pool, caplog):
    def completion(**kwargs):
        raise RuntimeError("provider said: invalid key test-key")

    caplog.set_level(logging.DEBUG)
    drafts = make_drafter(pool, "m", "test-key", 5, completion)(requests(1))
    assert drafts["X-0"].failure == "error"
    assert drafts["X-0"].data is None
    assert drafts["X-0"].attempts == 1  # not a provider error: no retry
    assert "test-key" not in str([record.__dict__ for record in caplog.records])
    assert [r.error for r in caplog.records if r.msg == "llm call failed"] == ["RuntimeError"]


@pytest.mark.parametrize("content", ["not json", "[1, 2]", '"text"', None])
def test_non_object_content_is_invalid_json(pool, content):
    drafts = make_drafter(pool, "m", "k", 5, answering(content))(requests(1))
    assert drafts["X-0"].failure == "invalid_json"
    assert drafts["X-0"].attempts == 2  # one extra attempt for invalid output


def test_slow_call_times_out_and_analysis_goes_on(pool):
    release = threading.Event()

    def completion(**kwargs):
        release.wait(5)
        return response(json.dumps(GOOD))

    started = time.monotonic()
    drafts = make_drafter(pool, "m", "k", 0.2, completion)(requests(1))
    elapsed = time.monotonic() - started
    release.set()
    assert drafts["X-0"].failure == "timeout"
    assert elapsed < 1.0


def test_queued_calls_are_cancelled_at_the_deadline():
    single = ThreadPoolExecutor(max_workers=1)
    release = threading.Event()
    calls = []

    def completion(**kwargs):
        calls.append(kwargs)
        release.wait(5)
        return response(json.dumps(GOOD))

    drafts = make_drafter(single, "m", "k", 0.2, completion)(requests(3))
    release.set()
    single.shutdown(wait=True)
    assert {d.failure for d in drafts.values()} == {"timeout"}
    assert len(calls) == 1  # the two queued calls never started


def test_concurrent_analyses_share_the_cap():
    shared = ThreadPoolExecutor(max_workers=2)
    lock = threading.Lock()
    in_flight, peak = 0, 0

    def completion(**kwargs):
        nonlocal in_flight, peak
        with lock:
            in_flight += 1
            peak = max(peak, in_flight)
        time.sleep(0.05)
        with lock:
            in_flight -= 1
        return response(json.dumps(GOOD))

    drafter = make_drafter(shared, "m", "k", 5, completion)
    results = []
    analyses = [
        threading.Thread(target=lambda p=prefix: results.append(drafter(requests(4, p))))
        for prefix in ("A", "B", "C")
    ]
    for thread in analyses:
        thread.start()
    for thread in analyses:
        thread.join()
    shared.shutdown(wait=True)
    assert peak == 2
    assert len(results) == 3
    assert all(d.data == GOOD for drafts in results for d in drafts.values())


def test_shut_down_pool_gives_error_drafts():
    closed = ThreadPoolExecutor(max_workers=1)
    closed.shutdown()
    drafts = make_drafter(closed, "m", "k", 1, answering(json.dumps(GOOD)))(requests(2))
    assert {d.failure for d in drafts.values()} == {"error"}


def test_draft_records_duration(pool):
    drafts = make_drafter(pool, "m", "k", 5, answering(json.dumps(GOOD)))(requests(1))
    assert drafts["X-0"].duration_ms >= 0


def test_no_pool_when_llm_disabled():
    assert create_llm_pool(load_settings(BASE_ENV)) is None
    assert create_llm_pool(load_settings({**BASE_ENV, "LLM_MODEL": "gemini/x"})) is None


def test_pool_size_follows_settings():
    settings = load_settings(
        {**BASE_ENV, "LLM_MODEL": "gemini/x", "LLM_API_KEY": "k", "LLM_MAX_CONCURRENCY": "3"}
    )
    llm_pool = create_llm_pool(settings)
    try:
        assert isinstance(llm_pool, ThreadPoolExecutor)
        assert llm_pool._max_workers == 3
    finally:
        llm_pool.shutdown()


def test_litellm_logs_go_through_the_root_handler_at_warning():
    for name in ("LiteLLM", "LiteLLM Router", "LiteLLM Proxy"):
        logger = logging.getLogger(name)
        assert logger.level == logging.WARNING
        assert logger.handlers == []
        assert logger.propagate is True
    assert logging.getLogger("httpx").level == logging.WARNING


PROMPT = "Medidor: X-0\nHECHOS:\n- variación: +10,0 %"
GROUNDED = {"reason": "Sube +10,0 %.", "explanation": "Sube +10,0 %.", "action_detail": "Revisar."}
UNGROUNDED = {**GROUNDED, "explanation": "Sube +99,0 %."}


def gemini_error(kind, **kwargs):
    return kind("provider problem", llm_provider="gemini", model="m", **kwargs)


def scripted(*outcomes):
    """Completion that returns (or raises) the given outcomes in order; records calls."""
    calls = []

    def completion(**kwargs):
        calls.append(kwargs)
        outcome = outcomes[len(calls) - 1]
        if isinstance(outcome, Exception):
            raise outcome
        return response(json.dumps(outcome))

    return completion, calls


def draft_with(completion, timeout=20, sleeps=None, analysis_id=None):
    single = ThreadPoolExecutor(max_workers=1)
    pause = (lambda seconds: sleeps.append(seconds)) if sleeps is not None else (lambda s: None)
    drafter = make_drafter(single, "m", "k", timeout, completion, sleep=pause)
    try:
        return drafter([DraftRequest("X-0", PROMPT, analysis_id)])["X-0"]
    finally:
        single.shutdown(wait=True)


@pytest.mark.parametrize(
    ("exc", "expected"),
    [
        (gemini_error(litellm.ServiceUnavailableError), True),
        (gemini_error(litellm.InternalServerError), True),
        (gemini_error(litellm.RateLimitError), True),
        (gemini_error(litellm.APIConnectionError), True),
        (litellm.Timeout("slow", model="m", llm_provider="gemini"), True),
        (gemini_error(litellm.NotFoundError), False),
        (gemini_error(litellm.BadRequestError), False),
        (gemini_error(litellm.AuthenticationError), False),
        (RuntimeError("bug"), False),
    ],
)
def test_retryable_errors(exc, expected):
    assert retryable(exc) is expected


def test_overloaded_provider_is_retried_with_backoff(caplog):
    overloaded = gemini_error(litellm.ServiceUnavailableError)
    completion, calls = scripted(overloaded, overloaded, GROUNDED)
    sleeps = []
    caplog.set_level(logging.INFO)
    draft = draft_with(completion, sleeps=sleeps, analysis_id=5)
    assert (draft.data, draft.failure, draft.attempts) == (GROUNDED, None, 3)
    assert len(calls) == 3
    assert 0.8 <= sleeps[0] <= 1.2 and 1.6 <= sleeps[1] <= 2.4
    retries = [
        (r.meter_id, r.attempt, r.reason, r.analysis_id)
        for r in caplog.records
        if r.msg == "llm_retry"
    ]
    assert retries == [
        ("X-0", 2, "ServiceUnavailableError", 5),
        ("X-0", 3, "ServiceUnavailableError", 5),
    ]


def test_transient_errors_stop_after_two_retries():
    overloaded = gemini_error(litellm.InternalServerError)
    completion, calls = scripted(overloaded, overloaded, overloaded, GROUNDED)
    draft = draft_with(completion)
    assert (draft.failure, draft.attempts, len(calls)) == ("error", 3, 3)


def test_last_per_call_timeout_is_reported_as_timeout():
    slow = litellm.Timeout("slow", model="m", llm_provider="gemini")
    completion, _ = scripted(slow, slow, slow)
    assert draft_with(completion).failure == "timeout"


def test_not_found_is_not_retried():
    completion, calls = scripted(gemini_error(litellm.NotFoundError), GROUNDED)
    draft = draft_with(completion)
    assert (draft.failure, draft.attempts, len(calls)) == ("error", 1, 1)


def test_ungrounded_draft_gets_one_more_attempt(caplog):
    completion, calls = scripted(UNGROUNDED, GROUNDED)
    caplog.set_level(logging.INFO)
    draft = draft_with(completion)
    assert (draft.data, draft.attempts, len(calls)) == (GROUNDED, 2, 2)
    assert [r.reason for r in caplog.records if r.msg == "llm_retry"] == ["ungrounded_number"]


def test_ungrounded_twice_is_left_for_review_to_reject():
    completion, calls = scripted(UNGROUNDED, UNGROUNDED, GROUNDED)
    draft = draft_with(completion)
    assert (draft.data, draft.attempts, len(calls)) == (UNGROUNDED, 2, 2)
    assert validate(draft.data, PROMPT)[1] == "ungrounded_number"


def test_rejected_final_draft_falls_back_to_template():
    events = make_events((3, "X-1", "2026-09-12 14:00", "UNKNOWN", "No operational event"))
    facts = build_facts(make_anomaly(related_event_ids=[3]), events)
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    bad = {"reason": "Sube 999 %.", "explanation": "x", "action_detail": "y"}
    completion, calls = scripted(bad, bad)
    single = ThreadPoolExecutor(max_workers=1)
    draft = make_drafter(single, "m", "k", 20, completion, sleep=lambda s: None)(
        [DraftRequest("X-1", prompt)]
    )["X-1"]
    single.shutdown(wait=True)
    result = explain(facts, prompt, draft)
    assert (result.source, result.rejection, len(calls)) == ("template", "ungrounded_number", 2)


def test_no_retry_when_the_deadline_is_too_close():
    completion, calls = scripted(gemini_error(litellm.ServiceUnavailableError), GROUNDED)
    sleeps = []
    draft = draft_with(completion, timeout=1.0, sleeps=sleeps)
    assert (draft.failure, draft.attempts, len(calls), sleeps) == ("timeout", 1, 1, [])


def test_invalid_output_retry_skipped_when_the_deadline_is_too_close():
    completion, calls = scripted(UNGROUNDED, GROUNDED)
    draft = draft_with(completion, timeout=1.0)
    assert (draft.failure, draft.attempts, len(calls)) == ("timeout", 1, 1)


def test_retry_after_is_honored():
    limited = gemini_error(litellm.RateLimitError, headers={"retry-after": "0.5"})
    completion, _ = scripted(limited, GROUNDED)
    sleeps = []
    draft = draft_with(completion, sleeps=sleeps)
    assert (draft.data, draft.attempts, sleeps) == (GROUNDED, 2, [0.5])


def test_retry_after_beyond_the_deadline_gives_up_immediately():
    limited = gemini_error(litellm.RateLimitError, headers={"retry-after": "30"})
    completion, calls = scripted(limited, GROUNDED)
    sleeps = []
    draft = draft_with(completion, timeout=5.0, sleeps=sleeps)
    assert (draft.failure, draft.attempts, len(calls), sleeps) == ("timeout", 1, 1, [])


def test_concurrency_cap_holds_with_retries():
    shared = ThreadPoolExecutor(max_workers=2)
    lock = threading.Lock()
    in_flight, peak, seen = 0, 0, set()

    def completion(**kwargs):
        nonlocal in_flight, peak
        meter = kwargs["messages"][1]["content"]
        with lock:
            in_flight += 1
            peak = max(peak, in_flight)
            first = meter not in seen
            seen.add(meter)
        time.sleep(0.02)
        with lock:
            in_flight -= 1
        if first:
            raise gemini_error(litellm.ServiceUnavailableError)
        return response(json.dumps(GOOD))

    drafter = make_drafter(shared, "m", "k", 20, completion, sleep=lambda s: time.sleep(0.01))
    results = []
    analyses = [
        threading.Thread(target=lambda p=prefix: results.append(drafter(requests(3, p))))
        for prefix in ("A", "B", "C")
    ]
    for thread in analyses:
        thread.start()
    for thread in analyses:
        thread.join()
    shared.shutdown(wait=True)
    assert peak == 2
    drafts = [d for batch in results for d in batch.values()]
    assert len(drafts) == 9
    assert all(d.data == GOOD and d.attempts == 2 for d in drafts)

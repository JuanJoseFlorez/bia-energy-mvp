"""Accept an LLM draft only if it is well-formed and cites nothing absent from its input."""

import re
from collections.abc import Callable
from dataclasses import dataclass

from pydantic import BaseModel, ConfigDict, ValidationError, field_validator

from app.explanation.facts import Facts, Tokens, tokens
from app.explanation.templates import recommended_action, template_texts

REASON_MAX = 160
EXPLANATION_MAX = 600
ACTION_DETAIL_MAX = 200
EPSILON = 1e-9  # float slack on top of the precision tolerance below

LLM = "llm"
TEMPLATE = "template"


@dataclass(frozen=True)
class DraftRequest:
    meter_id: str
    prompt: str  # user message; also the grounding reference
    analysis_id: int | None = None  # log correlation only


@dataclass(frozen=True)
class Draft:
    data: dict | None = None  # JSON object returned by the LLM
    failure: str | None = None  # "timeout" | "error" | "invalid_json" | "missing"
    duration_ms: float = 0.0
    attempts: int = 0  # LLM calls made for this draft


Drafter = Callable[[list[DraftRequest]], dict[str, Draft]]


@dataclass(frozen=True)
class Explanation:
    reason: str
    explanation: str
    recommended_action: str
    source: str  # LLM | TEMPLATE
    rejection: str | None = None  # why a requested draft was not used


class DraftText(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True, str_strip_whitespace=True)

    reason: str
    explanation: str
    action_detail: str

    @field_validator("reason", "explanation", "action_detail")
    @classmethod
    def not_empty(cls, value: str) -> str:
        if not value:
            raise ValueError("empty text")
        return value


def _too_long(text: DraftText) -> bool:
    return (
        len(text.reason) > REASON_MAX
        or len(text.explanation) > EXPLANATION_MAX
        or len(text.action_detail) > ACTION_DETAIL_MAX
    )


def _grounded(
    value: float | None,
    is_percent: bool,
    decimals: int,
    allowed_all: list[float],
    allowed_percent: list[float],
) -> bool:
    if value is None:
        return False
    # a number written with d decimal digits only claims precision to half its last digit
    tolerance = 0.5 * 10 ** (-decimals) + EPSILON
    if is_percent:  # the sign of a percentage may be expressed in words ("cayó un 79,7 %")
        # percentages must match percentages in the prompt
        return any(abs(abs(value) - abs(a)) <= tolerance for a in allowed_percent)
    # non-percentages match all numbers in the prompt
    return any(abs(value - a) <= tolerance for a in allowed_all)


def _strip_meter_id(text: str, meter_id: str) -> str:
    """Blank out the meter id where it stands as a whole token (case-insensitive).

    Not preceded by a word character, and not followed by one either -- nor by a "." or ","
    that itself precedes a digit, since that means the id is only a prefix of a longer
    number (e.g. "X-1" inside "X-10" or "X-1,5") rather than the id on its own.
    """
    pattern = re.compile(rf"(?<!\w){re.escape(meter_id)}(?!\w|[.,]\d)", re.IGNORECASE)
    return pattern.sub(" ", text)


def ungrounded(texts: list[str], reference: Tokens, meter_id: str | None = None) -> str | None:
    """Name of the first grounding failure (dates/times, then numbers), or None.

    Percentages in output are checked only against percentages in the prompt;
    other numbers are checked against all numbers in the prompt. Occurrences of the meter
    id itself (e.g. "el medidor X-109") are dropped first: naming the meter is not a claim
    about a number, and the id is not part of the allowed pool either (see `_reference`).
    """
    joined = "\n".join(texts)
    if meter_id:
        joined = _strip_meter_id(joined, meter_id)
    found = tokens(joined)
    if found.dates - reference.dates or found.times - reference.times:
        return "ungrounded_date"
    allowed_all = [value for value, _, _ in reference.numbers if value is not None]
    allowed_percent = [
        value for value, is_pct, _ in reference.numbers if value is not None and is_pct
    ]
    if not all(
        _grounded(value, pct, decimals, allowed_all, allowed_percent)
        for value, pct, decimals in found.numbers
    ):
        return "ungrounded_number"
    return None


def _reference(prompt: str) -> tuple[Tokens, str | None]:
    """Tokens the LLM may cite (the prompt without its `Medidor:` line) and the id on that line."""
    body_lines, meter_id = [], None
    for line in prompt.splitlines():
        if line.startswith("Medidor:"):
            meter_id = line.split(":", 1)[1].strip()
        else:
            body_lines.append(line)
    return tokens("\n".join(body_lines)), meter_id


def validate(data: dict, prompt: str) -> tuple[DraftText | None, str | None]:
    """Parsed texts, or the rejection: "schema", "too_long", "ungrounded_date/number"."""
    try:
        text = DraftText.model_validate(data)
    except ValidationError:
        return None, "schema"
    if _too_long(text):
        return None, "too_long"
    reference, meter_id = _reference(prompt)
    problem = ungrounded([text.reason, text.explanation, text.action_detail], reference, meter_id)
    if problem:
        return None, problem
    return text, None


def _check(draft: Draft, prompt: str) -> tuple[DraftText | None, str | None]:
    if draft.failure or draft.data is None:
        return None, draft.failure or "missing"
    return validate(draft.data, prompt)


def explain(facts: Facts, prompt: str, draft: Draft | None) -> Explanation:
    """LLM texts when the draft passes every check, template texts otherwise."""
    rejection = None
    if draft is not None:
        text, rejection = _check(draft, prompt)
        if text is not None:
            return Explanation(
                reason=text.reason,
                explanation=text.explanation,
                recommended_action=recommended_action(facts.type, text.action_detail),
                source=LLM,
            )
    fallback = template_texts(facts)
    return Explanation(
        reason=fallback.reason,
        explanation=fallback.explanation,
        recommended_action=recommended_action(facts.type, fallback.action_detail),
        source=TEMPLATE,
        rejection=rejection,
    )

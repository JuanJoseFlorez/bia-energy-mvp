from datetime import datetime

import pytest

from app.explanation.facts import build_facts, render_prompt
from app.explanation.review import Draft, explain
from app.explanation.templates import ACTION_BASE, template_texts
from tests.support import make_anomaly, make_events, no_events

CHANGE = make_anomaly(
    "REAL_ANOMALY",
    "HIGH",
    0.99,
    baseline_kwh=1052.7,
    current_kwh=2207.6,
    variation_pct=109.708,
    change_start=datetime(2026, 9, 12, 14),
    changed_vars={"power_factor": {"before": 0.94, "after": 0.74}},
    related_event_ids=[3],
    signals=["power_factor_drop"],
)
OUTAGE = make_anomaly(
    "FALSE_POSITIVE",
    "LOW",
    0.85,
    baseline_kwh=1348.075,
    current_kwh=1362.11,
    variation_pct=1.041,
    transient={
        "start": datetime(2026, 9, 8),
        "end": datetime(2026, 9, 8, 11),
        "hours": 12,
        "mean_deviation_pct": -79.729,
        "effect_size": -21.9,
    },
    related_event_ids=[2],
)
EVENTS = make_events(
    (2, "X-1", "2026-09-08 00:00", "SCHEDULED_OUTAGE", "Maintenance outage for 12 hours"),
    (3, "X-1", "2026-09-12 14:00", "UNKNOWN", "No operational event reported"),
)
FACTS = build_facts(CHANGE, EVENTS)
PROMPT = render_prompt(FACTS, ACTION_BASE["REAL_ANOMALY"])
GOOD = {
    "reason": "El consumo subió +109,7 % desde el 12-sep 14:00 sin causa operativa registrada.",
    "explanation": "El baseline diario era 1.052,7 kWh y el último día llegó a 2.207,6 kWh; "
    "el factor de potencia bajó de 0,94 a 0,74.",
    "action_detail": "Inspeccionar los equipos conectados desde el 12-sep 14:00.",
}


def review(data=None, failure=None, facts=FACTS, prompt=PROMPT):
    return explain(facts, prompt, Draft(data=data, failure=failure))


def with_explanation(text: str) -> dict:
    return {**GOOD, "explanation": text}


def test_valid_draft_is_used_and_action_is_anchored():
    result = review(GOOD)
    assert result.source == "llm"
    assert result.rejection is None
    assert result.reason == GOOD["reason"]
    assert result.explanation == GOOD["explanation"]
    assert result.recommended_action == (
        "Investigar medidor e instalación. Inspeccionar los equipos conectados desde el "
        "12-sep 14:00."
    )


def test_texts_are_stripped():
    result = review({**GOOD, "reason": "  " + GOOD["reason"] + "\n"})
    assert (result.source, result.reason) == ("llm", GOOD["reason"])


def test_no_draft_uses_templates_without_rejection():
    result = explain(FACTS, PROMPT, None)
    expected = template_texts(FACTS)
    assert result.source == "template"
    assert result.rejection is None
    assert result.reason == expected.reason
    assert result.explanation == expected.explanation
    assert (
        result.recommended_action == f"Investigar medidor e instalación. {expected.action_detail}"
    )


@pytest.mark.parametrize("failure", ["timeout", "error", "invalid_json", "missing"])
def test_failed_draft_falls_back(failure):
    result = review(failure=failure)
    assert (result.source, result.rejection) == ("template", failure)


def test_draft_without_data_counts_as_missing():
    assert review().rejection == "missing"


@pytest.mark.parametrize(
    "data",
    [
        {"reason": GOOD["reason"], "explanation": GOOD["explanation"]},
        {**GOOD, "reason": ""},
        {**GOOD, "action_detail": "   "},
        {**GOOD, "reason": 42},
        {**GOOD, "explanation": None},
        {**GOOD, "type": "FALSE_POSITIVE"},
    ],
    ids=["missing_field", "empty", "blank", "not_a_string", "null", "extra_key"],
)
def test_schema_violations_fall_back(data):
    result = review(data)
    assert (result.source, result.rejection) == ("template", "schema")


@pytest.mark.parametrize(
    "data",
    [
        {**GOOD, "reason": "a" * 161},
        with_explanation("b" * 601),
        {**GOOD, "action_detail": "c" * 201},
    ],
    ids=["reason", "explanation", "action_detail"],
)
def test_too_long_falls_back(data):
    assert review(data).rejection == "too_long"


def test_limits_are_inclusive():
    data = {"reason": "a" * 160, "explanation": "b" * 600, "action_detail": "c" * 200}
    assert review(data).source == "llm"


@pytest.mark.parametrize(
    ("text", "rejection"),
    [
        ("El consumo subió +120,0 % sobre el baseline.", "ungrounded_number"),
        ("El consumo subió un 1.052,7 % adicional respecto al mes pasado.", "ungrounded_number"),
        ("El cambio dura ya 999 h.", "ungrounded_number"),
        ("El cambio empieza el 13-sep 14:00.", "ungrounded_date"),
        ("El cambio empieza el 12-sep 15:00.", "ungrounded_date"),
        ("Ocurre en 2026 desde el 12-sep 14:00.", "ungrounded_number"),
        ("El factor de potencia bajó 0,20 puntos.", "ungrounded_number"),
        ("El factor de potencia bajó de 0.94 a 0.74.", "ungrounded_number"),
        ("El baseline era 1.052,9 kWh.", "ungrounded_number"),
        # precision-based tolerance: a value written with 2 decimals only tolerates ±0.005
        ("El factor de potencia bajó de 0,94 a 0,84.", "ungrounded_number"),
        ("El efecto residual cayó a 0,65.", "ungrounded_number"),
        ("La confianza del motor es 0,90.", "ungrounded_number"),
        ("El consumo llegó a 2.207,5 kWh.", "ungrounded_number"),
    ],
)
def test_ungrounded_output_falls_back(text, rejection):
    result = review(with_explanation(text))
    assert (result.source, result.rejection) == ("template", rejection)


def test_ungrounded_number_in_action_detail_falls_back():
    data = {**GOOD, "action_detail": "Revisar las 3 líneas nuevas."}
    assert review(data).rejection == "ungrounded_number"


@pytest.mark.parametrize(
    "text",
    [
        "El baseline era 1052,7 kWh y ahora 2207,6 kWh.",
        "El baseline era 1.053 kWh.",
        "El factor de potencia antes era 0,9.",
        "El consumo subió 110 %.",
        "Subió 109,7% desde el 12-sep 14:00.",
        "Consumo −109,7 % respecto al baseline en el medidor X-1.",
        "La confianza del motor es 0,99.",
    ],
    ids=[
        "no_thousands_dot",
        "fewer_written_decimals",  # "1.053" (0 decimals) tolerates ±0.5 vs baseline 1.052,7
        "coarser_precision",  # "0,9" (1 decimal) tolerates ±0.05 vs power factor 0,94
        "percent_rounds",  # "110 %" (0 decimals) tolerates ±0.5 vs +109,7 %
        "percent_no_space",
        "minus_sign",
        "conf",
    ],
)
def test_grounded_variants_are_accepted(text):
    assert review(with_explanation(text)).source == "llm"


def test_meter_id_digits_are_not_part_of_the_grounding_pool():
    facts = build_facts(make_anomaly(meter_id="X-109"), no_events())
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    data = {
        "reason": "El consumo subió sin causa operativa registrada.",
        "explanation": "Subió 109 kWh según el registro del medidor.",
        "action_detail": "Inspeccionar el medidor y la instalación asociada.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("template", "ungrounded_number")


def test_draft_naming_its_own_meter_is_still_grounded():
    facts = build_facts(make_anomaly(meter_id="X-109", variation_pct=109.7), no_events())
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    data = {
        "reason": "El medidor X-109 muestra un alza sostenida.",
        "explanation": "El medidor X-109 subió +109,7 % respecto al baseline.",
        "action_detail": "Inspeccionar el medidor X-109 y la instalación asociada.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("llm", None)


def test_meter_id_is_stripped_only_as_a_whole_token_not_a_prefix():
    # meter id "X-1" must not swallow the unrelated "X-10": stripping it naively as a
    # substring would turn "X-10" into a spurious "0" instead of leaving "10" intact.
    facts = build_facts(make_anomaly(meter_id="X-1", current_kwh=10.0), no_events())
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    data = {
        "reason": "Revisar el equipo X-10 junto al medidor.",
        "explanation": "El equipo X-10 registra el mismo consumo del último día.",
        "action_detail": "Contactar al responsable del equipo X-10.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("llm", None)


def test_meter_id_followed_by_comma_and_digit_is_not_mangled():
    # "X-1,5" must not be split at the id boundary either: a naive substring strip of
    # "X-1" would leave " ,5", from which only a spurious "5" gets tokenized.
    facts = build_facts(make_anomaly(meter_id="X-1", current_kwh=1.5), no_events())
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    data = {
        "reason": "El consumo del último día coincide con la referencia X-1,5.",
        "explanation": "La referencia X-1,5 iguala el consumo del último día.",
        "action_detail": "Confirmar la referencia X-1,5 en el informe técnico.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("llm", None)


def test_meter_id_is_stripped_case_insensitively():
    facts = build_facts(make_anomaly(meter_id="X-109", variation_pct=109.7), no_events())
    prompt = render_prompt(facts, ACTION_BASE["REAL_ANOMALY"])
    data = {
        "reason": "el medidor x-109 muestra un alza sostenida.",
        "explanation": "el medidor x-109 subió +109,7 % respecto al baseline.",
        "action_detail": "inspeccionar el medidor x-109 y la instalación asociada.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("llm", None)


def test_integers_from_the_input_are_accepted():
    facts = build_facts(OUTAGE, EVENTS)
    prompt = render_prompt(facts, ACTION_BASE["FALSE_POSITIVE"])
    data = {
        "reason": "Caída de 12 h (-79,7 %) durante la parada programada del 08-sep 00:00.",
        "explanation": "Entre el 08-sep 00:00 y el 08-sep 11:00 el consumo cayó un 79,7 % "
        "y después volvió a 1.362,1 kWh.",
        "action_detail": "Archivar el caso junto al registro de la parada de 12 horas.",
    }
    result = review(data, facts=facts, prompt=prompt)
    assert (result.source, result.rejection) == ("llm", None)
    assert result.recommended_action.startswith("No escalar. ")


@pytest.mark.parametrize("anomaly", [CHANGE, OUTAGE])
def test_template_texts_are_grounded(anomaly):
    facts = build_facts(anomaly, EVENTS)
    prompt = render_prompt(facts, ACTION_BASE[anomaly.type])
    t = template_texts(facts)
    data = {"reason": t.reason, "explanation": t.explanation, "action_detail": t.action_detail}
    assert review(data, facts=facts, prompt=prompt).source == "llm"

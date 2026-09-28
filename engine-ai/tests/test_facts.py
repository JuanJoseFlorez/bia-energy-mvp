from datetime import datetime

import pytest

from app.explanation.facts import (
    CHECK_LABELS,
    NO_EVENTS,
    build_facts,
    event_label,
    moment,
    number,
    percent,
    render_prompt,
    tokens,
)
from tests.support import make_anomaly, make_events, no_events

M109_LIKE = {
    "baseline_kwh": 1052.7,
    "current_kwh": 2207.6,
    "variation_pct": 109.70836895601784,
    "change_start": datetime(2026, 9, 12, 14),
    "changed_vars": {
        "power_factor": {"before": 0.94, "after": 0.74},
        "current_a": {"before": 195.35, "after": 411.07},
        "voltage_v": {"before": 219.835, "after": 216.985},
    },
    "related_event_ids": [3],
    "signals": ["no_explaining_event", "power_factor_drop"],
}
EVENTS = make_events(
    (1, "X-2", "2026-09-11 00:00", "OPERATIONAL_CHANGE", "New production line activated"),
    (3, "X-1", "2026-09-12 14:00", "UNKNOWN", "No operational event reported"),
)


@pytest.mark.parametrize(
    ("value", "decimals", "expected"),
    [
        (1052.7, 1, "1.052,7"),
        (2207.604, 1, "2.207,6"),
        (1234567.25, 1, "1.234.567,3"),
        (195.35, 1, "195,4"),
        (216.985, 1, "217,0"),
        (0.9412, 2, "0,94"),
        (0.885, 2, "0,89"),
        (-79.72931, 1, "-79,7"),
        (-1052.7, 1, "-1.052,7"),
        (-0.04, 1, "0,0"),
        (12, 0, "12"),
    ],
)
def test_number_spanish_format(value, decimals, expected):
    assert number(value, decimals) == expected


def test_percent_has_sign_one_decimal_and_space():
    assert percent(109.70836) == "+109,7 %"
    assert percent(-79.72931) == "-79,7 %"
    assert percent(0.034) == "+0,0 %"
    assert percent(-0.02) == "+0,0 %"


def test_moment_uses_spanish_months():
    assert moment(datetime(2026, 9, 12, 14)) == "12-sep 14:00"
    assert moment(datetime(2026, 1, 3, 9, 5)) == "03-ene 09:05"
    months = [moment(datetime(2026, m, 1)).split()[0][3:] for m in range(1, 13)]
    assert months == [
        "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"
    ]  # fmt: skip


def test_labels():
    assert event_label("OPERATIONAL_CHANGE") == "cambio operativo"
    assert event_label("SCHEDULED_OUTAGE") == "parada programada"
    assert event_label("DATA_QUALITY") == "reporte de calidad de datos"
    assert event_label("UNKNOWN") == "sin evento operativo conocido"
    assert event_label("SOMETHING_NEW") == "evento no clasificado"
    assert CHECK_LABELS == {
        "voltage_out_of_range": "voltaje fuera de rango",
        "voltage_jump": "saltos de voltaje",
        "current_jump_flat_consumption": "saltos de corriente con consumo plano",
        "power_ratio_inconsistent": "kWh inconsistente con V·I·PF",
        "power_factor_jump": "saltos de factor de potencia",
        "power_factor_out_of_range": "factor de potencia fuera de [0, 1]",
        "flatline": "valores repetidos",
    }


@pytest.mark.parametrize(
    ("kind", "severity", "type_label", "severity_label"),
    [
        ("REAL_ANOMALY", "HIGH", "anomalía real", "alta"),
        ("DATA_QUALITY", "HIGH", "calidad de datos", "alta"),
        ("EXPLAINABLE_ANOMALY", "MEDIUM", "anomalía explicable", "media"),
        ("FALSE_POSITIVE", "LOW", "falso positivo", "baja"),
    ],
)
def test_type_and_severity_labels(kind, severity, type_label, severity_label):
    facts = build_facts(make_anomaly(kind, severity), no_events())
    assert (facts.type_label, facts.severity_label) == (type_label, severity_label)


def test_facts_of_a_persistent_change():
    facts = build_facts(make_anomaly(confidence=0.99, **M109_LIKE), EVENTS)
    assert facts.lines() == [
        ("tipo", "anomalía real"),
        ("severidad", "alta"),
        ("confianza", "0,99"),
        ("baseline diario", "1.052,7 kWh"),
        ("consumo último día", "2.207,6 kWh"),
        ("variación", "+109,7 %"),
        ("inicio del cambio", "12-sep 14:00"),
        ("factor de potencia", "0,94 → 0,74"),
        ("corriente", "195,4 → 411,1 A"),
        ("voltaje", "219,8 → 217,0 V"),
        ("evento relacionado", "sin evento operativo conocido, 12-sep 14:00"),
    ]
    assert facts.power_factor_drop is True


def test_facts_of_transient_and_failed_checks():
    anomaly = make_anomaly(
        "FALSE_POSITIVE",
        "LOW",
        transient={
            "start": datetime(2026, 9, 8),
            "end": datetime(2026, 9, 8, 11),
            "hours": 12,
            "mean_deviation_pct": -79.7293,
            "effect_size": -21.9,
        },
        failed_checks=[{"check": "flatline", "hours": 7}],
    )
    lines = dict(build_facts(anomaly, no_events()).lines())
    assert lines["transitorio"] == "08-sep 00:00 a 08-sep 11:00, 12 h, desviación media -79,7 %"
    assert lines["chequeo fallido"] == "valores repetidos, 7 h"
    assert lines["eventos relacionados"] == NO_EVENTS == "ningún evento registrado"
    assert "inicio del cambio" not in lines


def test_description_reaches_only_the_prompt():
    facts = build_facts(make_anomaly(**M109_LIKE), EVENTS)
    prompt = render_prompt(facts, "Investigar medidor e instalación")
    assert "No operational event reported" in prompt
    assert "No operational event reported" not in str(facts.lines())
    assert "New production line activated" not in prompt  # event of another meter
    assert prompt.splitlines()[:3] == [
        "Medidor: X-1",
        "Acción base: Investigar medidor e instalación",
        "HECHOS:",
    ]
    assert prompt.splitlines()[-2:] == [
        "EVENTOS:",
        "- 12-sep 14:00 · sin evento operativo conocido · No operational event reported",
    ]


def test_prompt_without_events():
    prompt = render_prompt(build_facts(make_anomaly(), no_events()), "Validar operación")
    assert prompt.endswith("- eventos relacionados: ningún evento registrado\nEVENTOS:\n- ninguno")


@pytest.mark.parametrize("description", [None, float("nan"), "  "])
def test_missing_description_is_left_out(description):
    events = make_events((3, "X-1", "2026-09-12 14:00", "UNKNOWN", description))
    facts = build_facts(make_anomaly(related_event_ids=[3]), events)
    prompt = render_prompt(facts, "Investigar medidor e instalación")
    assert prompt.endswith("- 12-sep 14:00 · sin evento operativo conocido")


def test_allowed_tokens_come_from_the_whole_prompt():
    facts = build_facts(make_anomaly(confidence=0.99, **M109_LIKE), EVENTS)
    allowed = tokens(render_prompt(facts, "Investigar medidor e instalación"))
    assert allowed.dates == {"12-sep"}
    assert allowed.times == {"14:00"}
    values = {value for value, _, _ in allowed.numbers}
    assert {1052.7, 2207.6, 109.7, 0.94, 0.74, 195.4, 411.1, 219.8, 217.0, 0.99} <= values
    assert (109.7, True, 1) in allowed.numbers  # the variation is a percentage
    assert 1.0 in values  # the meter id "X-1" is part of the input too
    assert 2026.0 not in values  # no year anywhere in the input


def test_tokens_parse_spanish_numbers_dates_and_times():
    found = tokens("Subió +109,7 % (1.052,7 → 1052,7 kWh) el 12-Sep 14:00 en M-7, bajó -3,5.")
    assert found.dates == {"12-sep"}
    assert found.times == {"14:00"}
    assert found.numbers == (
        (109.7, True, 1),
        (1052.7, False, 1),
        (1052.7, False, 1),
        (7.0, False, 0),
        (-3.5, False, 1),
    )


def test_tokens_expose_written_decimal_digits():
    # thousands dots are not decimals: "1.053" has 0 written decimals, "0,94" has 2
    found = tokens("1.053 y 0,94 y 110 %")
    assert found.numbers == ((1053.0, False, 0), (0.94, False, 2), (110.0, True, 0))


def test_tokens_mark_english_decimals_unparseable():
    assert tokens("pf 0.94").numbers == ((None, False, 0),)

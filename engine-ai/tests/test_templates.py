from datetime import datetime

from app.explanation.facts import build_facts
from app.explanation.templates import ACTION_BASE, recommended_action, template_texts
from tests.support import make_anomaly, make_events, no_events

M109 = make_anomaly(
    "REAL_ANOMALY",
    "HIGH",
    0.99,
    baseline_kwh=1052.7,
    current_kwh=2207.6,
    variation_pct=109.708,
    change_start=datetime(2026, 9, 12, 14),
    changed_vars={
        "power_factor": {"before": 0.94, "after": 0.74},
        "current_a": {"before": 195.35, "after": 411.07},
        "voltage_v": {"before": 219.835, "after": 216.985},
    },
    related_event_ids=[3],
    signals=["no_explaining_event", "power_factor_drop"],
)
M112 = make_anomaly(
    "DATA_QUALITY",
    "HIGH",
    0.99,
    baseline_kwh=662.425,
    current_kwh=662.65,
    variation_pct=0.034,
    failed_checks=[
        {"check": "voltage_out_of_range", "hours": 16},
        {"check": "voltage_jump", "hours": 31},
    ],
    related_event_ids=[4],
)
M104 = make_anomaly(
    "EXPLAINABLE_ANOMALY",
    "MEDIUM",
    0.83,
    baseline_kwh=1167.59,
    current_kwh=1725.63,
    variation_pct=47.794,
    change_start=datetime(2026, 9, 11),
    changed_vars={
        "power_factor": {"before": 0.907, "after": 0.889},
        "current_a": {"before": 213.31, "after": 315.455},
    },
    related_event_ids=[1],
)
M106 = make_anomaly(
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
    (1, "X-1", "2026-09-11 00:00", "OPERATIONAL_CHANGE", "New production line activated"),
    (2, "X-1", "2026-09-08 00:00", "SCHEDULED_OUTAGE", "Scheduled maintenance outage"),
    (3, "X-1", "2026-09-12 14:00", "UNKNOWN", "No operational event reported"),
    (4, "X-1", "2026-09-13 00:00", "DATA_QUALITY", "Intermittent readings"),
)


def texts_of(anomaly, events=EVENTS):
    return template_texts(build_facts(anomaly, events))


def test_action_base_per_type():
    assert ACTION_BASE == {
        "REAL_ANOMALY": "Investigar medidor e instalación",
        "DATA_QUALITY": "Validar medidor / lecturas",
        "EXPLAINABLE_ANOMALY": "Validar operación",
        "FALSE_POSITIVE": "No escalar",
    }
    assert recommended_action("FALSE_POSITIVE", "Registrar.") == "No escalar. Registrar."


def test_real_anomaly_texts():
    t = texts_of(M109)
    assert t.reason == (
        "Consumo +109,7 % sobre el baseline desde el 12-sep 14:00, sin evento operativo que "
        "lo explique."
    )
    assert t.explanation == (
        "El consumo diario pasó de un baseline de 1.052,7 kWh a 2.207,6 kWh (+109,7 %). "
        "El cambio empieza el 12-sep 14:00 y persiste hasta el final de los datos. "
        "Cambiaron también las variables eléctricas: factor de potencia 0,94 → 0,74, "
        "corriente 195,4 → 411,1 A, voltaje 219,8 → 217,0 V. "
        "El único evento registrado es 'sin evento operativo conocido', que no explica el cambio."
    )
    assert t.action_detail == (
        "Revisar la carga conectada desde el 12-sep 14:00 y la caída del factor de potencia."
    )


def test_data_quality_texts():
    t = texts_of(M112)
    assert t.reason == "Consumo estable (+0,0 %) con lecturas eléctricas inconsistentes."
    assert "662,7 kWh" in t.explanation and "662,4 kWh" in t.explanation
    assert "voltaje fuera de rango (16 h)" in t.explanation
    assert "saltos de voltaje (31 h)" in t.explanation
    assert "reporte de calidad de datos (13-sep 00:00)" in t.explanation
    assert t.action_detail == "Revisar el medidor y su comunicación antes de usar estos datos."


def test_explainable_texts():
    t = texts_of(M104)
    assert t.reason == (
        "Consumo +47,8 % desde el 11-sep 00:00, coincide con un cambio operativo registrado."
    )
    assert "1.167,6 kWh a 1.725,6 kWh (+47,8 %)" in t.explanation
    assert "corriente 213,3 → 315,5 A" in t.explanation
    assert "factor de potencia 0,91 → 0,89" in t.explanation
    assert "cambio operativo (11-sep 00:00)" in t.explanation
    assert t.action_detail == (
        "Confirmar con operación que la nueva carga es la esperada y actualizar el baseline."
    )


def test_false_positive_texts():
    t = texts_of(M106)
    assert t.reason == "Caída transitoria de 12 h (-79,7 %) dentro de una parada programada."
    assert "entre el 08-sep 00:00 y el 08-sep 11:00 (12 h)" in t.explanation
    assert "1.362,1 kWh" in t.explanation and "(+1,0 %)" in t.explanation
    assert "parada programada (08-sep 00:00)" in t.explanation
    assert t.action_detail == "Registrar como explicado por la parada programada."


def test_missing_optional_evidence_omits_sentences():
    bare = make_anomaly("REAL_ANOMALY", "MEDIUM", variation_pct=-35.0)
    t = texts_of(bare, no_events())
    assert t.reason == "Consumo -35,0 % bajo el baseline, sin evento operativo que lo explique."
    assert t.explanation == (
        "El consumo diario pasó de un baseline de 1.000,0 kWh a 1.500,0 kWh (-35,0 %)."
    )
    assert t.action_detail == "Revisar la carga conectada."
    for text in (t.reason, t.explanation, t.action_detail):
        assert "None" not in text and "  " not in text and ": ." not in text


def test_real_transient_without_event():
    transient = dict(M106.evidence.transient, mean_deviation_pct=-45.0, hours=5)
    t = texts_of(make_anomaly("REAL_ANOMALY", "MEDIUM", transient=transient), no_events())
    assert (
        t.reason == "Desviación transitoria de 5 h (-45,0 %) sin evento operativo que la explique."
    )
    assert "entre el 08-sep 00:00 y el 08-sep 11:00 (5 h)" in t.explanation
    assert "evento" not in t.explanation
    assert t.action_detail == (
        "Revisar qué ocurrió en la instalación entre el 08-sep 00:00 y el 08-sep 11:00."
    )


def test_several_unexplaining_events():
    two = make_events(
        (3, "X-1", "2026-09-12 14:00", "UNKNOWN", ""),
        (5, "X-1", "2026-09-12 16:00", "METER_SWAP", ""),
    )
    anomaly = make_anomaly(
        change_start=datetime(2026, 9, 12, 14), related_event_ids=[3, 5], signals=[]
    )
    t = texts_of(anomaly, two)
    assert t.explanation.endswith(
        "Los eventos registrados ('sin evento operativo conocido', 'evento no clasificado') "
        "no explican el cambio."
    )
    assert t.action_detail == "Revisar la carga conectada desde el 12-sep 14:00."

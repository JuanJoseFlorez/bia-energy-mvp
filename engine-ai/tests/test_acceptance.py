"""Acceptance tests on the real seed data (db-init/*.csv)."""

import pytest

from app.analysis.detectors import data_quality_flags
from app.analysis.pipeline import establish_reference, prepare_series, run_analysis

EXPECTED = {
    "M-109": ("REAL_ANOMALY", "HIGH"),
    "M-112": ("DATA_QUALITY", "HIGH"),
    "M-104": ("EXPLAINABLE_ANOMALY", "MEDIUM"),
    "M-106": ("FALSE_POSITIVE", "LOW"),
}
MIN_CONFIDENCE = {"M-109": 0.90, "M-112": 0.85, "M-104": 0.80, "M-106": 0.60}


@pytest.fixture(scope="module")
def result(real_readings, real_events) -> dict:
    return list(run_analysis(real_readings, real_events))[-1]


@pytest.fixture(scope="module")
def by_meter(result) -> dict[str, dict]:
    return {a["meter_id"]: a for a in result["anomalies"]}


def test_expected_types_and_severities(by_meter):
    assert {m: (a["type"], a["severity"]) for m, a in by_meter.items()} == EXPECTED


def test_other_meters_are_normal(result, by_meter):
    normal = {m["meter_id"] for m in result["metrics"]} - set(by_meter)
    assert len(normal) == 8


def test_normal_meters_have_no_data_quality_flags(real_readings):
    series = prepare_series(real_readings)
    normal = {meter_id: s for meter_id, s in series.items() if meter_id not in EXPECTED}
    assert len(normal) == 8
    for meter_id, s in normal.items():
        ref, _ = establish_reference(s)
        assert not data_quality_flags(s, ref).to_numpy().any(), meter_id


def test_priority_order(result):
    assert [(a["meter_id"], a["priority"]) for a in result["anomalies"]] == [
        ("M-109", 1),
        ("M-112", 2),
        ("M-104", 3),
        ("M-106", 4),
    ]


def test_high_priority_count(result):
    high = [a for a in result["anomalies"] if a["severity"] == "HIGH" and a["anomaly"]]
    assert len(result["anomalies"]) == 4
    assert len(high) == 2


def test_false_positive_is_not_an_anomaly(by_meter):
    assert by_meter["M-106"]["anomaly"] is False
    assert all(by_meter[m]["anomaly"] for m in ("M-109", "M-112", "M-104"))


def test_confidence_ranges(by_meter):
    for meter_id, floor in MIN_CONFIDENCE.items():
        assert floor <= by_meter[meter_id]["confidence"] <= 0.99, meter_id


def test_metrics_for_all_meters(result):
    assert len(result["metrics"]) == 12


def test_m109_metrics_and_evidence(result, by_meter, real_events):
    m109 = next(m for m in result["metrics"] if m["meter_id"] == "M-109")
    assert m109["variation_pct"] > 100
    assert m109["change_start"] == "2026-09-12T14:00:00Z"
    evidence = by_meter["M-109"]["evidence"]
    event_id = int(real_events.loc[real_events["meter_id"] == "M-109", "id"].iloc[0])
    assert event_id in evidence["related_event_ids"]
    assert evidence["failed_checks"] == []
    assert {"no_explaining_event", "power_factor_drop"} <= set(evidence["signals"])


def test_m112_failed_checks(by_meter):
    checks = {c["check"] for c in by_meter["M-112"]["evidence"]["failed_checks"]}
    assert "voltage_out_of_range" in checks


def test_m106_transient_inside_outage(by_meter):
    transient = by_meter["M-106"]["evidence"]["transient"]
    assert transient["start"] == "2026-09-08T00:00:00Z"
    assert transient["hours"] == 12


def test_deterministic(real_readings, real_events, result):
    again = list(run_analysis(real_readings, real_events))[-1]
    assert again == result

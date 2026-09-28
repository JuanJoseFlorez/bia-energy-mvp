import json
import logging
import math

import pandas as pd
import pytest

from app.analysis.pipeline import establish_reference, run_analysis
from app.explanation.review import Draft
from tests.support import SHAPE, START, as_readings, make_series, no_events

STEPS = [
    "READINGS",
    "BASELINE",
    "DETECTION",
    "CORRELATION",
    "EVENTS",
    "EXPLANATION",
    "RECOMMENDATION",
]
CORE_FIELDS = ("meter_id", "anomaly", "type", "severity", "confidence", "priority", "evidence")


def result_of(readings: pd.DataFrame, events: pd.DataFrame | None = None) -> dict:
    out = list(run_analysis(readings, no_events() if events is None else events))
    return out[-1]


def stepped(at: str, factor: float) -> pd.DataFrame:
    s = make_series()
    later = s["timestamp"] >= pd.Timestamp(at)
    s.loc[later, ["consumption_kwh", "current_a"]] *= factor
    return s


def test_steps_in_order_then_result():
    out = list(run_analysis(as_readings(make_series()), no_events()))
    assert [e["type"] for e in out] == ["step"] * 7 + ["result"]
    assert [e["step"] for e in out[:7]] == STEPS


def test_two_pass_reference_uses_pre_change_days():
    s = stepped("2026-09-04 12:00", 1.8)
    ref, change = establish_reference(s)
    assert change is not None
    assert change.start == pd.Timestamp("2026-09-04 12:00")
    assert ref.reliable
    assert ref.window.sum() == 3 * 24
    assert ref.baseline_kwh == pytest.approx(40.0 * SHAPE.sum(), rel=0.02)


def test_change_on_first_day_marks_baseline_unreliable():
    s = stepped("2026-09-01 18:00", 1.8)
    result = result_of(as_readings(s))
    [anomaly] = result["anomalies"]
    assert anomaly["type"] == "REAL_ANOMALY"
    assert anomaly["evidence"]["baseline_reliable"] is False
    assert anomaly["evidence"]["change_start"] == "2026-09-01T18:00:00Z"
    reliable = result_of(as_readings(stepped("2026-09-10 00:00", 1.8)))["anomalies"][0]
    assert anomaly["confidence"] == round(reliable["confidence"] - 0.15, 2)


def test_noise_free_series_uses_scale_floors():
    # Every day identical and no value repeated 6 hours in a row: every MAD is 0.
    n = 14 * 24
    ts = pd.date_range(START, periods=n, freq="h")
    series = pd.DataFrame(
        {
            "timestamp": ts,
            "consumption_kwh": 40.0 * SHAPE[ts.hour] + 0.01 * ts.hour,
            "voltage_v": 220.0 + 0.1 * (ts.hour % 3),
            "current_a": 180.0 * SHAPE[ts.hour] + 0.1 * (ts.hour % 2),
            "power_factor": 0.93 + 0.001 * (ts.hour % 3),
        }
    )
    ref, change = establish_reference(series)
    assert change is None
    assert ref.sigma == 0.01
    result = result_of(as_readings(series))
    assert result["anomalies"] == []
    [metrics] = result["metrics"]
    assert all(math.isfinite(metrics[k]) for k in ("current_kwh", "baseline_kwh", "variation_pct"))
    assert metrics["variation_pct"] == 0.0


def test_meter_with_fewer_than_48_readings_is_skipped():
    short = as_readings(make_series().head(47), "X-2")
    result = result_of(pd.concat([as_readings(make_series(), "X-1"), short]))
    assert [m["meter_id"] for m in result["metrics"]] == ["X-1"]


def test_meter_with_zero_baseline_is_skipped():
    zero = make_series()
    zero["consumption_kwh"] = 0.0
    result = result_of(pd.concat([as_readings(make_series(), "X-1"), as_readings(zero, "X-2")]))
    assert [m["meter_id"] for m in result["metrics"]] == ["X-1"]
    assert result["anomalies"] == []


def test_result_is_strict_json():
    s = stepped("2026-09-10 00:00", 1.8)
    s.loc[s["timestamp"] >= "2026-09-10 00:00", "power_factor"] = 0.75
    result = result_of(as_readings(s))
    text = json.dumps(result, allow_nan=False)
    anomaly = json.loads(text)["anomalies"][0]
    assert anomaly["type"] == "REAL_ANOMALY"
    assert anomaly["severity"] == "HIGH"
    assert anomaly["priority"] == 1
    assert anomaly["anomaly"] is True
    assert set(anomaly["evidence"]["changed_vars"]) == {"power_factor", "current_a", "voltage_v"}


def test_event_of_another_meter_explains_nothing():
    events = pd.DataFrame(
        {
            "id": [1],
            "meter_id": ["X-2"],
            "event_timestamp": [pd.Timestamp("2026-09-10 00:00")],
            "event_type": ["OPERATIONAL_CHANGE"],
        }
    )
    readings = pd.concat(
        [as_readings(stepped("2026-09-10 00:00", 1.5), "X-1"), as_readings(make_series(), "X-2")]
    )
    [anomaly] = result_of(readings, events)["anomalies"]
    assert (anomaly["meter_id"], anomaly["type"]) == ("X-1", "REAL_ANOMALY")
    assert anomaly["evidence"]["related_event_ids"] == []


def test_data_quality_evidence_does_not_mask_real_anomaly():
    s = stepped("2026-09-10 00:00", 2.0)
    bump = (s["timestamp"] >= "2026-09-12 00:00") & (s["timestamp"] < "2026-09-12 03:00")
    s.loc[bump, "voltage_v"] *= 1.10
    [anomaly] = result_of(as_readings(s))["anomalies"]
    assert anomaly["type"] == "REAL_ANOMALY"
    failed_checks = anomaly["evidence"]["failed_checks"]
    assert failed_checks
    assert "voltage_out_of_range" in {c["check"] for c in failed_checks}


def test_unsorted_duplicated_and_missing_rows_are_cleaned():
    clean = as_readings(stepped("2026-09-10 00:00", 1.5))
    messy = pd.concat([clean.iloc[::-1], clean.iloc[[5]]])  # reversed + duplicate hour
    broken = clean.iloc[[7]].assign(consumption_kwh=float("nan"))
    messy = pd.concat([messy[messy["timestamp"] != broken["timestamp"].iloc[0]], broken])
    expected = result_of(clean.drop(index=7))
    assert result_of(messy) == expected


def two_anomalies() -> pd.DataFrame:
    return pd.concat(
        [
            as_readings(stepped("2026-09-10 00:00", 1.8), "X-1"),
            as_readings(stepped("2026-09-11 00:00", 1.4), "X-2"),
            as_readings(make_series(), "X-3"),
        ]
    )


def core(result: dict) -> list[dict]:
    return [{k: a[k] for k in CORE_FIELDS} for a in result["anomalies"]]


def test_without_drafter_every_anomaly_gets_template_texts():
    anomalies = result_of(two_anomalies())["anomalies"]
    assert [a["meter_id"] for a in anomalies] == ["X-1", "X-2"]
    for a in anomalies:
        assert a["explanation_source"] == "template"
        assert a["reason"] and a["explanation"]
        assert a["recommended_action"].startswith("Investigar medidor e instalación. ")


def test_drafter_changes_texts_but_never_the_classification():
    requested = []

    def drafter(requests):
        requested.extend(requests)
        return {
            r.meter_id: Draft(
                data={
                    "reason": "Texto del modelo.",
                    "explanation": "Explicación del modelo.",
                    "action_detail": "Paso concreto.",
                }
            )
            for r in requests
        }

    readings = two_anomalies()
    with_llm = list(run_analysis(readings, no_events(), drafter=drafter))[-1]
    without = result_of(readings)
    assert core(with_llm) == core(without)
    assert with_llm["metrics"] == without["metrics"]
    assert [r.meter_id for r in requested] == ["X-1", "X-2"]
    assert requested[0].prompt.startswith("Medidor: X-1\nAcción base: Investigar")
    for a in with_llm["anomalies"]:
        assert a["explanation_source"] == "llm"
        assert a["reason"] == "Texto del modelo."
        assert a["recommended_action"] == "Investigar medidor e instalación. Paso concreto."


def test_rejected_and_missing_drafts_fall_back_per_anomaly(caplog):
    def drafter(requests):
        assert {r.analysis_id for r in requests} == {9}  # for the llm_retry log lines
        return {
            "X-1": Draft(data={"reason": "Sube 999 %.", "explanation": "x", "action_detail": "y"})
        }

    caplog.set_level(logging.INFO)
    out = list(run_analysis(two_anomalies(), no_events(), analysis_id=9, drafter=drafter))
    anomalies = out[-1]["anomalies"]
    assert [a["explanation_source"] for a in anomalies] == ["template", "template"]
    fallbacks = [
        (r.meter_id, r.reason, r.analysis_id) for r in caplog.records if r.msg == "llm_fallback"
    ]
    assert fallbacks == [("X-1", "ungrounded_number", 9), ("X-2", "missing", 9)]
    [summary] = [r for r in caplog.records if r.msg == "explanations ready"]
    assert (summary.llm, summary.template) == (0, 2)


def test_failing_drafter_never_breaks_the_analysis():
    def drafter(requests):
        raise RuntimeError("boom")

    out = list(run_analysis(two_anomalies(), no_events(), drafter=drafter))
    assert out[-1]["type"] == "result"
    assert {a["explanation_source"] for a in out[-1]["anomalies"]} == {"template"}


def test_drafter_is_not_called_without_anomalies():
    calls = []
    out = list(run_analysis(as_readings(make_series()), no_events(), drafter=calls.append))
    assert out[-1]["anomalies"] == []
    assert calls == []

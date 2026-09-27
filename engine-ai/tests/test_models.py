from datetime import datetime

from app.analysis.models import Anomaly, Evidence, MeterMetrics


def test_metrics_to_dict_rounds_and_formats():
    m = MeterMetrics("X-1", 2207.604, 1050.4949, 110.1512, datetime(2026, 9, 12, 14))
    assert m.to_dict() == {
        "meter_id": "X-1",
        "current_kwh": 2207.6,
        "baseline_kwh": 1050.49,
        "variation_pct": 110.15,
        "change_start": "2026-09-12T14:00:00Z",
    }
    assert MeterMetrics("X-1", 1, 1, 0, None).to_dict()["change_start"] is None


def test_anomaly_to_dict():
    evidence = Evidence(
        baseline_kwh=10.0,
        current_kwh=20.0,
        variation_pct=100.0,
        change_start=None,
        baseline_reliable=True,
        changed_vars={"power_factor": {"before": 0.9412, "after": 0.7389}},
        transient={
            "start": datetime(2026, 9, 8),
            "end": datetime(2026, 9, 8, 11),
            "hours": 12,
            "mean_deviation_pct": -79.731,
            "effect_size": -21.886,
        },
        outlier_timestamps=[datetime(2026, 9, 8, 1)],
    )
    out = Anomaly("X-1", "FALSE_POSITIVE", "LOW", 0.854, evidence, priority=4).to_dict()
    assert out["anomaly"] is False
    assert out["confidence"] == 0.85
    assert out["priority"] == 4
    ev = out["evidence"]
    assert ev["changed_vars"] == {"power_factor": {"before": 0.94, "after": 0.74}}
    assert ev["transient"]["start"] == "2026-09-08T00:00:00Z"
    assert ev["transient"]["mean_deviation_pct"] == -79.73
    assert ev["outlier_timestamps"] == ["2026-09-08T01:00:00Z"]
    assert ev["failed_checks"] == [] and ev["related_event_ids"] == [] and ev["signals"] == []
    assert ev["shift_pct"] is None and ev["profile_correlation"] is None


def test_only_false_positive_is_not_an_anomaly():
    evidence = Evidence(1.0, 1.0, 0.0, None, True)
    for kind in ("REAL_ANOMALY", "EXPLAINABLE_ANOMALY", "DATA_QUALITY"):
        assert Anomaly("X-1", kind, "HIGH", 0.9, evidence).anomaly is True

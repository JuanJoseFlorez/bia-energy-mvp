import pandas as pd
import pytest

from app.analysis.baseline import build_reference
from app.analysis.detectors import (
    detect_outliers,
    detect_persistent_change,
    detect_transient,
    profile_correlation,
)
from tests.support import make_series


def ref_of(series: pd.DataFrame):
    return build_reference(series, pd.Series(True, index=series.index), reliable=True)


def step(series: pd.DataFrame, at: str, factor: float) -> pd.DataFrame:
    """Consumption and current both change by factor from `at` on."""
    after = series["timestamp"] >= pd.Timestamp(at)
    series.loc[after, ["consumption_kwh", "current_a"]] *= factor
    return series


def test_persistent_change_detects_step_in_middle_of_window():
    # Half the data after the change: the pooled sigma is inflated, the segment scale is not.
    s = step(make_series(), "2026-09-08 00:00", 1.5)
    change = detect_persistent_change(s, ref_of(s))
    assert change is not None
    assert change.start == pd.Timestamp("2026-09-08 00:00")
    assert change.shift_pct == pytest.approx(0.5, abs=0.05)
    assert change.effect >= 5
    assert change.post_hours == 7 * 24


def test_persistent_change_ignores_normal_series():
    s = make_series()
    assert detect_persistent_change(s, ref_of(s)) is None


def test_persistent_change_ignores_small_step():
    s = step(make_series(), "2026-09-08 00:00", 1.1)
    assert detect_persistent_change(s, ref_of(s)) is None


def test_persistent_change_needs_24_hours_after_split():
    s = step(make_series(), "2026-09-14 04:00", 2.0)  # 20 hours before the end
    assert detect_persistent_change(s, ref_of(s)) is None


def test_transient_detects_run_that_recovers():
    s = make_series()
    outage = (s["timestamp"] >= "2026-09-08 00:00") & (s["timestamp"] <= "2026-09-08 11:00")
    s.loc[outage, "consumption_kwh"] *= 0.2
    tr = detect_transient(s, ref_of(s))
    assert tr is not None
    assert (tr.start, tr.end, tr.hours) == (
        pd.Timestamp("2026-09-08 00:00"),
        pd.Timestamp("2026-09-08 11:00"),
        12,
    )
    assert tr.mean_deviation == pytest.approx(-0.8, abs=0.03)
    assert tr.effect < -5


def test_transient_ignores_normal_series():
    s = make_series()
    assert detect_transient(s, ref_of(s)) is None


def test_transient_ignores_runs_of_24_hours_or_more():
    s = make_series()
    long_dip = (s["timestamp"] >= "2026-09-08 00:00") & (s["timestamp"] < "2026-09-09 02:00")
    s.loc[long_dip, "consumption_kwh"] *= 0.2
    assert detect_transient(s, ref_of(s)) is None


def test_transient_needs_recovery():
    s = make_series()
    s.loc[s.index[-5:], "consumption_kwh"] *= 0.2  # dip runs until the end of the data
    assert detect_transient(s, ref_of(s)) is None


def test_outliers_flag_spike_only():
    s = make_series()
    s.loc[100, "consumption_kwh"] *= 3
    ref = ref_of(s)
    assert detect_outliers(s, ref) == [s.loc[100, "timestamp"]]
    clean = make_series()
    assert detect_outliers(clean, ref_of(clean)) == []


def test_profile_correlation():
    s = make_series()
    assert profile_correlation(s, ref_of(s)) > 0.95
    s.loc[s.index[-24:], "consumption_kwh"] = 40.0  # flat last day: shape undefined
    assert profile_correlation(s, ref_of(s)) is None

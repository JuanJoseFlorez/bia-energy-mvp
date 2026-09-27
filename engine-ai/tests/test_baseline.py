import math

import pandas as pd
import pytest

from app.analysis.baseline import (
    build_reference,
    daily_metrics,
    pre_change_window,
    robust_scale,
)
from tests.support import SHAPE, make_series


def all_rows(series: pd.DataFrame) -> pd.Series:
    return pd.Series(True, index=series.index)


def test_ratio_removes_daily_shape():
    s = make_series(noise=0.02)
    ref = build_reference(s, all_rows(s), reliable=True)
    assert ref.ratio.median() == pytest.approx(1.0, abs=0.01)
    assert 0.01 < ref.sigma < 0.04
    assert ref.profile[12] / ref.profile[2] == pytest.approx(1.55, rel=0.03)


def test_robust_scale_uses_floor_for_constant_values():
    assert robust_scale(pd.Series([1.0] * 50), floor=0.01) == 0.01
    assert robust_scale(pd.Series([], dtype=float), floor=0.5) == 0.5


def test_zero_profile_hour():
    s = make_series()
    s.loc[s["timestamp"].dt.hour == 3, "consumption_kwh"] = 0.0
    s.loc[10 * 24 + 3, "consumption_kwh"] = 5.0  # one non-zero reading at a zero-profile hour
    ref = build_reference(s, all_rows(s), reliable=True)
    assert ref.ratio[3] == 1.0  # 0 kWh at a 0 profile hour compares as normal
    assert math.isnan(ref.ratio[10 * 24 + 3])  # excluded from ratio-based detectors
    assert ref.ratio.notna().sum() == len(s) - 1


def test_physical_ratio_reference_is_own_median():
    s = make_series()
    ref = build_reference(s, all_rows(s), reliable=True)
    assert ref.r_ref == pytest.approx(ref.physical_ratio.median())
    assert ref.voltage_ref == pytest.approx(220, abs=0.5)
    assert ref.current_norm.median() == pytest.approx(1.0, abs=0.01)


def test_pre_change_window_uses_full_days_before_change():
    s = make_series()
    window = pre_change_window(s, pd.Timestamp("2026-09-04 12:00"))
    assert window is not None
    assert window.sum() == 3 * 24
    assert s.loc[window, "timestamp"].max() == pd.Timestamp("2026-09-03 23:00")


def test_pre_change_window_too_short():
    s = make_series()
    assert pre_change_window(s, pd.Timestamp("2026-09-03 12:00")) is None


def test_daily_metrics():
    s = make_series(noise=0.0, level=10.0)
    ref = build_reference(s, all_rows(s), reliable=True)
    s.loc[s.index[-24:], "consumption_kwh"] *= 1.5
    m = daily_metrics("X-1", s, ref, None)
    day = 10.0 * SHAPE.sum()
    assert ref.baseline_kwh == pytest.approx(day)
    assert m.current_kwh == pytest.approx(1.5 * day)
    assert m.variation_pct == pytest.approx(50.0)
    assert m.change_start is None


def test_baseline_is_median_of_daily_totals():
    s = make_series(noise=0.0, level=10.0)
    s.loc[7 * 24 : 7 * 24 + 11, "consumption_kwh"] *= 0.2  # a 12-hour outage on day 8
    ref = build_reference(s, all_rows(s), reliable=True)
    assert ref.baseline_kwh == pytest.approx(10.0 * SHAPE.sum())  # the outage day does not count

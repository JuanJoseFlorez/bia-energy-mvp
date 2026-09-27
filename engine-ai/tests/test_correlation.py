import pandas as pd
import pytest

from app.analysis.baseline import build_reference
from app.analysis.detectors import (
    DQ_CHECKS,
    data_quality_flags,
    detect_data_quality,
    detect_electrical_change,
    detect_persistent_change,
)
from tests.support import make_series


def ref_of(series: pd.DataFrame):
    return build_reference(series, pd.Series(True, index=series.index), reliable=True)


def step(series: pd.DataFrame, at: str, factor: float) -> pd.DataFrame:
    """Consumption and current both change by factor from `at` on."""
    after = series["timestamp"] >= pd.Timestamp(at)
    series.loc[after, ["consumption_kwh", "current_a"]] *= factor
    return series


def test_electrical_change_with_power_factor_drop():
    s = step(make_series(), "2026-09-08 00:00", 2.0)
    s.loc[s["timestamp"] >= "2026-09-08 00:00", "power_factor"] = 0.75
    ref = ref_of(s)
    change = detect_persistent_change(s, ref)
    e = detect_electrical_change(s, ref, change)
    assert e.fired
    assert e.signals == ("power_factor_drop", "power_ratio_shift", "current_follows")
    assert e.changed_vars["power_factor"]["before"] == pytest.approx(0.93, abs=0.01)
    assert e.changed_vars["power_factor"]["after"] == pytest.approx(0.75)
    assert e.power_ratio_shift == pytest.approx(0.93 / 0.75 - 1, abs=0.03)


def test_electrical_change_not_fired_for_consistent_load_increase():
    s = step(make_series(), "2026-09-08 00:00", 1.5)
    ref = ref_of(s)
    e = detect_electrical_change(s, ref, detect_persistent_change(s, ref))
    assert not e.fired
    assert e.signals == ("current_follows",)
    assert set(e.changed_vars) == {"power_factor", "current_a", "voltage_v"}


def rows(first: int, count: int, every: int = 1) -> list[int]:
    return list(range(first, first + count * every, every))


def glitch_voltage_high(s):
    s.loc[rows(240, 4), "voltage_v"] = 240.0  # +9 % for 4 hours


def glitch_voltage_jumps(s):
    s.loc[rows(240, 4, every=2), "voltage_v"] += 10.0  # +4.5 %: in range, but jumps


def glitch_current_jumps(s):
    s.loc[rows(240, 4, every=2), "current_a"] *= 1.5  # consumption untouched


def glitch_power_ratio(s):
    s.loc[rows(240, 4), "current_a"] *= 0.4  # V*I*PF no longer explains kWh


def glitch_power_factor_jumps(s):
    s.loc[rows(240, 4, every=2), "power_factor"] = 0.80


def glitch_power_factor_range(s):
    s.loc[rows(240, 3), "power_factor"] = 1.05


def glitch_flatline(s):
    s.loc[rows(240, 8), "consumption_kwh"] = 40.0


@pytest.mark.parametrize(
    ("check", "glitch"),
    [
        ("voltage_out_of_range", glitch_voltage_high),
        ("voltage_jump", glitch_voltage_jumps),
        ("current_jump_flat_consumption", glitch_current_jumps),
        ("power_ratio_inconsistent", glitch_power_ratio),
        ("power_factor_jump", glitch_power_factor_jumps),
        ("power_factor_out_of_range", glitch_power_factor_range),
        ("flatline", glitch_flatline),
    ],
)
def test_each_check_fires(check, glitch):
    s = make_series()
    glitch(s)
    dq = detect_data_quality(s, ref_of(s))
    assert dq is not None
    assert check in dq.failed_checks
    assert dq.onset >= s.loc[240, "timestamp"]
    assert dq.flagged_hours >= 3


def test_clean_series_has_no_flags():
    s = make_series()
    flags = data_quality_flags(s, ref_of(s))
    assert list(flags.columns) == list(DQ_CHECKS)
    assert not flags.to_numpy().any()
    assert detect_data_quality(s, ref_of(s)) is None


def test_fewer_than_three_flagged_hours_is_not_data_quality():
    s = make_series()
    s.loc[240, "voltage_v"] = 240.0  # flags hour 240 (range, jump up) and 241 (jump down)
    flags = data_quality_flags(s, ref_of(s))
    assert flags.any(axis=1).sum() == 2
    assert detect_data_quality(s, ref_of(s)) is None


def test_flagged_hours_spread_over_days_are_not_data_quality():
    s = make_series()
    for row in (72, 144, 216):  # one 2-hour glitch every 3 days
        s.loc[row, "voltage_v"] = 240.0
    assert data_quality_flags(s, ref_of(s)).any(axis=1).sum() == 6
    assert detect_data_quality(s, ref_of(s)) is None


def test_consumption_step_is_not_a_current_jump():
    s = make_series()
    later = s["timestamp"] >= "2026-09-08 00:00"
    s.loc[later, ["consumption_kwh", "current_a"]] *= 2.0  # load really doubles
    assert detect_data_quality(s, ref_of(s)) is None

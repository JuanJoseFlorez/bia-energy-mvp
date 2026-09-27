"""Reference window, hour-of-day profile, ratios and daily metrics for one meter."""

from dataclasses import dataclass

import pandas as pd

from app.analysis import thresholds as t
from app.analysis.models import MeterMetrics


@dataclass(frozen=True)
class Reference:
    """Baseline statistics of one meter, computed over the reference window."""

    window: pd.Series  # bool mask over the series rows used as reference
    profile: pd.Series  # median kWh per hour of day (index 0..23)
    ratio: pd.Series  # kWh / profile[hour]; NaN where the hour cannot be compared
    sigma: float  # pooled robust scale of ratio over the window
    physical_ratio: pd.Series  # kWh / (V * I * PF / 1000)
    r_ref: float  # median physical ratio over the window
    current_norm: pd.Series  # current / median current of that hour of day
    voltage_ref: float  # median voltage over the window
    baseline_kwh: float  # median daily total over the full days of the window
    reliable: bool


def robust_scale(values: pd.Series, floor: float) -> float:
    """1.4826 * MAD, never below floor (constant series never divide by zero)."""
    clean = values.dropna()
    if clean.empty:
        return floor
    mad = float((clean - clean.median()).abs().median())
    return max(t.MAD_TO_SIGMA * mad, floor)


def full_day_totals(series: pd.DataFrame, window: pd.Series) -> pd.Series:
    """Daily kWh totals of the days that have all 24 hours inside the window."""
    rows = series[window]
    days = rows["timestamp"].dt.floor("D")
    grouped = rows.groupby(days)["consumption_kwh"]
    counts = grouped.size()
    return grouped.sum()[counts == t.HOURS_PER_DAY]


def build_reference(series: pd.DataFrame, window: pd.Series, reliable: bool) -> Reference:
    """Compute profile, ratios and robust scales of a meter over the given window."""
    kwh = series["consumption_kwh"]
    hours = series["timestamp"].dt.hour
    ref_rows = series[window]
    ref_hours = hours[window]

    profile = ref_rows.groupby(ref_hours)["consumption_kwh"].median()
    expected = hours.map(profile)
    ratio = (kwh / expected.where(expected != 0)).astype(float)
    ratio = ratio.mask((expected == 0) & (kwh == 0), 1.0)

    apparent = series["voltage_v"] * series["current_a"] * series["power_factor"] / 1000
    physical = (kwh / apparent.where(apparent != 0)).astype(float)

    current_profile = ref_rows.groupby(ref_hours)["current_a"].median()
    expected_current = hours.map(current_profile)
    current_norm = (series["current_a"] / expected_current.where(expected_current != 0)).astype(
        float
    )

    totals = full_day_totals(series, window)
    return Reference(
        window=window,
        profile=profile,
        ratio=ratio,
        sigma=robust_scale(ratio[window], t.RATIO_SCALE_FLOOR),
        physical_ratio=physical,
        r_ref=float(physical[window].median()),
        current_norm=current_norm,
        voltage_ref=float(series["voltage_v"][window].median()),
        baseline_kwh=float(totals.median()) if not totals.empty else 0.0,
        reliable=reliable,
    )


def pre_change_window(series: pd.DataFrame, change_start: pd.Timestamp) -> pd.Series | None:
    """Full days strictly before change_start, or None if fewer than MIN_REFERENCE_DAYS."""
    days = series["timestamp"].dt.floor("D")
    window = days + pd.Timedelta(days=1) <= change_start
    if len(full_day_totals(series, window)) < t.MIN_REFERENCE_DAYS:
        return None
    return window


def daily_metrics(
    meter_id: str, series: pd.DataFrame, ref: Reference, change_start: pd.Timestamp | None
) -> MeterMetrics:
    """Last 24 hours vs typical daily consumption of the reference window."""
    current = float(series["consumption_kwh"].tail(t.HOURS_PER_DAY).sum())
    variation = (current - ref.baseline_kwh) / ref.baseline_kwh * 100
    return MeterMetrics(
        meter_id=meter_id,
        current_kwh=current,
        baseline_kwh=ref.baseline_kwh,
        variation_pct=float(variation),
        change_start=None if change_start is None else change_start.to_pydatetime(),
    )

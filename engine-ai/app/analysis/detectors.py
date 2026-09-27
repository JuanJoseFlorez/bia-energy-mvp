"""Detectors A-F: consumption changes, outliers, hourly pattern, electrical change, data quality."""

from dataclasses import dataclass

import numpy as np
import pandas as pd

from app.analysis import thresholds as t
from app.analysis.baseline import Reference, robust_scale


@dataclass(frozen=True)
class PersistentChange:
    start: pd.Timestamp
    shift_pct: float  # fraction: median(post) / median(pre) - 1
    effect: float  # (median(post) - median(pre)) / residual scale of both segments
    post_hours: int


@dataclass(frozen=True)
class Transient:
    start: pd.Timestamp
    end: pd.Timestamp
    hours: int
    mean_deviation: float  # fraction, mean of ratio - 1 over the run
    effect: float  # mean_deviation / sigma


def detect_persistent_change(series: pd.DataFrame, ref: Reference) -> PersistentChange | None:
    """A. Best single mean-shift split of the ratio; None unless it is large and persistent."""
    valid = ref.ratio.dropna()
    n = len(valid)
    if n < 2 * t.MIN_SEGMENT_HOURS:
        return None
    x = valid.to_numpy(dtype=float)
    csum = np.cumsum(x)
    csq = np.cumsum(x * x)
    k = np.arange(t.MIN_SEGMENT_HOURS, n - t.MIN_SEGMENT_HOURS + 1)
    left = csum[k - 1]
    right = csum[-1] - left
    sse = csq[-1] - left**2 / k - right**2 / (n - k)
    split = int(k[int(np.argmin(sse))])

    pre = float(np.median(x[:split]))
    post = float(np.median(x[split:]))
    shift = (post - pre) / max(abs(pre), t.RATIO_SCALE_FLOOR)
    # Scale of the residuals around each segment's median: unaffected by the step itself.
    residuals = pd.Series(np.concatenate([x[:split] - pre, x[split:] - post]))
    effect = (post - pre) / robust_scale(residuals, t.RATIO_SCALE_FLOOR)
    post_hours = n - split
    if (
        abs(shift) < t.MIN_CHANGE_SHIFT
        or abs(effect) < t.MIN_CHANGE_EFFECT
        or post_hours < t.MIN_POST_CHANGE_HOURS
    ):
        return None
    start = series["timestamp"].iloc[valid.index[split]]
    return PersistentChange(start=start, shift_pct=shift, effect=effect, post_hours=post_hours)


def detect_transient(series: pd.DataFrame, ref: Reference) -> Transient | None:
    """B. Longest run of |ratio - 1| >= 30% lasting 3..23 h that recovers for >= 3 h."""
    deviation = (ref.ratio - 1).to_numpy(dtype=float)
    outside = np.abs(deviation) >= t.TRANSIENT_DEVIATION  # NaN compares False
    inside = np.abs(deviation) < t.TRANSIENT_DEVIATION
    best: tuple[int, int] | None = None
    i, n = 0, len(deviation)
    while i < n:
        if not outside[i]:
            i += 1
            continue
        j = i
        while j + 1 < n and outside[j + 1]:
            j += 1
        length = j - i + 1
        recovery = inside[j + 1 : j + 1 + t.TRANSIENT_RECOVERY_HOURS]
        recovered = len(recovery) == t.TRANSIENT_RECOVERY_HOURS and bool(recovery.all())
        if t.MIN_TRANSIENT_HOURS <= length < t.MAX_TRANSIENT_HOURS and recovered:
            if best is None or length > best[1] - best[0] + 1:
                best = (i, j)
        i = j + 1
    if best is None:
        return None
    start, end = best
    mean_dev = float(np.mean(deviation[start : end + 1]))
    timestamps = series["timestamp"]
    return Transient(
        start=timestamps.iloc[start],
        end=timestamps.iloc[end],
        hours=end - start + 1,
        mean_deviation=mean_dev,
        effect=mean_dev / ref.sigma,
    )


def detect_outliers(series: pd.DataFrame, ref: Reference) -> list[pd.Timestamp]:
    """C. Hours with |ratio - 1| / sigma >= OUTLIER_Z (evidence only)."""
    z = (ref.ratio - 1).abs() / ref.sigma
    return list(series["timestamp"][z >= t.OUTLIER_Z])


def profile_correlation(series: pd.DataFrame, ref: Reference) -> float | None:
    """D. Pearson correlation of the last-24 h shape with the reference hour profile."""
    last = series.tail(t.HOURS_PER_DAY)
    expected = last["timestamp"].dt.hour.map(ref.profile).to_numpy(dtype=float)
    actual = last["consumption_kwh"].to_numpy(dtype=float)
    if np.isnan(expected).any() or np.std(actual) == 0 or np.std(expected) == 0:
        return None
    return float(np.corrcoef(actual / actual.mean(), expected / expected.mean())[0, 1])


# CORRELATION step: electrical change (E) and data quality (F)

DQ_CHECKS = (
    "voltage_out_of_range",
    "voltage_jump",
    "current_jump_flat_consumption",
    "power_ratio_inconsistent",
    "power_factor_jump",
    "power_factor_out_of_range",
    "flatline",
)


@dataclass(frozen=True)
class ElectricalChange:
    changed_vars: dict[str, dict[str, float]]
    power_factor_drop: float
    power_ratio_shift: float
    fired: bool
    signals: tuple[str, ...]


@dataclass(frozen=True)
class DataQuality:
    onset: pd.Timestamp
    flagged_hours: int
    failed_checks: dict[str, int]  # check name -> flagged hours, in DQ_CHECKS order


def detect_electrical_change(
    series: pd.DataFrame, ref: Reference, change: PersistentChange
) -> ElectricalChange:
    """E. Compare electrical variables before/after the persistent-change split."""
    before = series["timestamp"] < change.start
    after = ~before
    changed_vars = {
        col: {
            "before": float(series.loc[before, col].median()),
            "after": float(series.loc[after, col].median()),
        }
        for col in ("power_factor", "current_a", "voltage_v")
    }
    pf_drop = changed_vars["power_factor"]["before"] - changed_vars["power_factor"]["after"]
    r_before = float(ref.physical_ratio[before].median())
    r_after = float(ref.physical_ratio[after].median())
    r_shift = r_after / r_before - 1 if r_before else 0.0
    current_delta = changed_vars["current_a"]["after"] - changed_vars["current_a"]["before"]

    signals = []
    if pf_drop >= t.POWER_FACTOR_DROP:
        signals.append("power_factor_drop")
    if abs(r_shift) >= t.POWER_RATIO_SHIFT:
        signals.append("power_ratio_shift")
    if current_delta != 0 and np.sign(current_delta) == np.sign(change.shift_pct):
        signals.append("current_follows")
    fired = pf_drop >= t.POWER_FACTOR_DROP or abs(r_shift) >= t.POWER_RATIO_SHIFT
    return ElectricalChange(
        changed_vars=changed_vars,
        power_factor_drop=pf_drop,
        power_ratio_shift=r_shift,
        fired=fired,
        signals=tuple(signals),
    )


def _jump_z(values: pd.Series, window: pd.Series, floor: float) -> pd.Series:
    delta = values.diff()
    return delta.abs() / robust_scale(delta[window], floor)


def _relative_jump_z(values: pd.Series, window: pd.Series, floor: float) -> pd.Series:
    """Robust z of the relative hour-to-hour change, so a higher load level is not a jump."""
    previous = values.shift()
    change = values.diff() / previous.where(previous != 0)
    return change.abs() / robust_scale(change[window], floor)


def _flatline(values: pd.Series) -> pd.Series:
    run_id = (values != values.shift()).cumsum()
    run_length = values.groupby(run_id).transform("size")
    return run_length >= t.FLATLINE_HOURS


def data_quality_flags(series: pd.DataFrame, ref: Reference) -> pd.DataFrame:
    """F. One boolean column per check, one row per reading."""
    voltage = series["voltage_v"]
    pf = series["power_factor"]
    window = ref.window
    flags = pd.DataFrame(index=series.index)
    flags["voltage_out_of_range"] = (
        (voltage / ref.voltage_ref - 1).abs() > t.VOLTAGE_RANGE if ref.voltage_ref else False
    )
    flags["voltage_jump"] = _jump_z(voltage, window, t.VOLTAGE_JUMP_SCALE_FLOOR) > t.JUMP_Z
    flags["current_jump_flat_consumption"] = (
        _relative_jump_z(ref.current_norm, window, t.CURRENT_JUMP_SCALE_FLOOR) > t.JUMP_Z
    ) & (ref.ratio.diff().abs() < t.FLAT_CONSUMPTION_DELTA)
    flags["power_ratio_inconsistent"] = (
        (ref.physical_ratio / ref.r_ref - 1).abs() > t.POWER_RATIO_TOLERANCE if ref.r_ref else False
    )
    flags["power_factor_jump"] = _jump_z(pf, window, t.POWER_FACTOR_JUMP_SCALE_FLOOR) > t.JUMP_Z
    flags["power_factor_out_of_range"] = (pf < 0) | (pf > 1)
    flags["flatline"] = (
        _flatline(series["consumption_kwh"])
        | _flatline(voltage)
        | _flatline(series["current_a"])
        | _flatline(pf)
    )
    return flags[list(DQ_CHECKS)].fillna(False).astype(bool)


def detect_data_quality(series: pd.DataFrame, ref: Reference) -> DataQuality | None:
    """F. Fires when >= DQ_MIN_FLAGGED_HOURS flagged hours fall inside any 24 h window."""
    flags = data_quality_flags(series, ref)
    flagged = series["timestamp"][flags.any(axis=1)].reset_index(drop=True)
    span = pd.Timedelta(hours=t.DQ_WINDOW_HOURS)
    last = t.DQ_MIN_FLAGGED_HOURS - 1
    onset = next(
        (flagged[i] for i in range(len(flagged) - last) if flagged[i + last] - flagged[i] < span),
        None,
    )
    if onset is None:
        return None
    counts = flags.sum()
    return DataQuality(
        onset=onset,
        flagged_hours=len(flagged),
        failed_checks={name: int(counts[name]) for name in DQ_CHECKS if counts[name] > 0},
    )

"""Type, severity, confidence and priority from the detector findings (first rule wins)."""

from dataclasses import dataclass, replace

from app.analysis import thresholds as t
from app.analysis.detectors import DataQuality, ElectricalChange, PersistentChange, Transient
from app.analysis.events import EventRef, explains
from app.analysis.models import (
    DATA_QUALITY,
    EXPLAINABLE_ANOMALY,
    FALSE_POSITIVE,
    HIGH,
    LOW,
    MEDIUM,
    REAL_ANOMALY,
    Anomaly,
)

TYPE_RANK = {REAL_ANOMALY: 0, DATA_QUALITY: 1, EXPLAINABLE_ANOMALY: 2, FALSE_POSITIVE: 3}
SEVERITY_RANK = {HIGH: 0, MEDIUM: 1, LOW: 2}


@dataclass(frozen=True)
class Findings:
    change: PersistentChange | None = None
    transient: Transient | None = None
    electrical: ElectricalChange | None = None
    data_quality: DataQuality | None = None
    change_events: tuple[EventRef, ...] = ()
    transient_events: tuple[EventRef, ...] = ()
    data_quality_events: tuple[EventRef, ...] = ()
    baseline_reliable: bool = True


@dataclass(frozen=True)
class Classification:
    type: str
    severity: str
    confidence: float
    signals: tuple[str, ...]


def confidence(strength: float, signals: tuple[str, ...], baseline_reliable: bool) -> float:
    """min(0.99, 0.50 + 0.30 * strength + 0.05 * corroborations) - 0.15 if unreliable."""
    value = min(
        t.CONFIDENCE_MAX,
        t.CONFIDENCE_BASE
        + t.CONFIDENCE_STRENGTH_WEIGHT * strength
        + t.CONFIDENCE_PER_CORROBORATION * len(signals),
    )
    if not baseline_reliable:
        value -= t.UNRELIABLE_BASELINE_PENALTY
    return round(min(1.0, max(0.0, value)), 2)


def _effect_strength(effect: float) -> float:
    return min(1.0, abs(effect) / t.EFFECT_FOR_FULL_STRENGTH)


def classify(f: Findings) -> Classification | None:
    """Apply the classification rules; None means the meter is normal."""
    electrical = f.electrical.signals if f.electrical else ()
    if f.data_quality and not f.change:
        checks = tuple(f.data_quality.failed_checks)
        signals = checks[1:]
        if explains(f.data_quality_events, "data_quality"):
            signals += ("data_quality_event",)
        strength = min(1.0, f.data_quality.flagged_hours / t.DQ_HOURS_FOR_FULL_STRENGTH)
        kind, severity = DATA_QUALITY, HIGH
    elif f.change and explains(f.change_events, "persistent_change"):
        signals = ("explained_by_event",) + electrical
        strength = _effect_strength(f.change.effect)
        kind, severity = EXPLAINABLE_ANOMALY, MEDIUM
    elif f.change:
        signals = ("no_explaining_event",) + electrical
        strength = _effect_strength(f.change.effect)
        big = abs(f.change.shift_pct) >= t.HIGH_SEVERITY_SHIFT
        kind = REAL_ANOMALY
        severity = HIGH if big or (f.electrical and f.electrical.fired) else MEDIUM
    elif f.transient and explains(f.transient_events, "transient"):
        signals = ("explained_by_event",)
        strength = _effect_strength(f.transient.effect)
        kind, severity = FALSE_POSITIVE, LOW
    elif f.transient:
        signals = ()
        strength = _effect_strength(f.transient.effect)
        kind, severity = REAL_ANOMALY, MEDIUM
    else:
        return None
    return Classification(
        type=kind,
        severity=severity,
        confidence=confidence(strength, signals, f.baseline_reliable),
        signals=signals,
    )


def prioritize(anomalies: list[Anomaly]) -> list[Anomaly]:
    """Sort by type, severity, confidence, |variation|, meter_id and number 1..n."""
    ordered = sorted(
        anomalies,
        key=lambda a: (
            TYPE_RANK[a.type],
            SEVERITY_RANK[a.severity],
            -a.confidence,
            -abs(a.evidence.variation_pct),
            a.meter_id,
        ),
    )
    return [replace(a, priority=i) for i, a in enumerate(ordered, start=1)]

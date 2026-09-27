"""Output contract of the analysis: per-meter metrics and anomalies with evidence."""

from dataclasses import dataclass, field
from datetime import datetime

REAL_ANOMALY = "REAL_ANOMALY"
EXPLAINABLE_ANOMALY = "EXPLAINABLE_ANOMALY"
FALSE_POSITIVE = "FALSE_POSITIVE"
DATA_QUALITY = "DATA_QUALITY"

HIGH = "HIGH"
MEDIUM = "MEDIUM"
LOW = "LOW"


def format_ts(value: datetime | None) -> str | None:
    """RFC3339 UTC; timestamps are naive UTC throughout the engine."""
    return None if value is None else value.strftime("%Y-%m-%dT%H:%M:%SZ")


def round2(value: float | None) -> float | None:
    return None if value is None else round(float(value), 2)


@dataclass(frozen=True)
class MeterMetrics:
    meter_id: str
    current_kwh: float
    baseline_kwh: float
    variation_pct: float
    change_start: datetime | None

    def to_dict(self) -> dict:
        return {
            "meter_id": self.meter_id,
            "current_kwh": round2(self.current_kwh),
            "baseline_kwh": round2(self.baseline_kwh),
            "variation_pct": round2(self.variation_pct),
            "change_start": format_ts(self.change_start),
        }


@dataclass(frozen=True)
class Evidence:
    baseline_kwh: float
    current_kwh: float
    variation_pct: float
    change_start: datetime | None
    baseline_reliable: bool
    shift_pct: float | None = None
    effect_size: float | None = None
    changed_vars: dict[str, dict[str, float]] | None = None
    transient: dict | None = None
    failed_checks: list[dict] = field(default_factory=list)
    outlier_timestamps: list[datetime] = field(default_factory=list)
    profile_correlation: float | None = None
    related_event_ids: list[int] = field(default_factory=list)
    signals: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        changed = None
        if self.changed_vars is not None:
            changed = {
                name: {k: round2(v) for k, v in values.items()}
                for name, values in self.changed_vars.items()
            }
        transient = None
        if self.transient is not None:
            transient = {
                "start": format_ts(self.transient["start"]),
                "end": format_ts(self.transient["end"]),
                "hours": self.transient["hours"],
                "mean_deviation_pct": round2(self.transient["mean_deviation_pct"]),
                "effect_size": round2(self.transient["effect_size"]),
            }
        return {
            "baseline_kwh": round2(self.baseline_kwh),
            "current_kwh": round2(self.current_kwh),
            "variation_pct": round2(self.variation_pct),
            "change_start": format_ts(self.change_start),
            "baseline_reliable": self.baseline_reliable,
            "shift_pct": round2(self.shift_pct),
            "effect_size": round2(self.effect_size),
            "changed_vars": changed,
            "transient": transient,
            "failed_checks": list(self.failed_checks),
            "outlier_timestamps": [format_ts(t) for t in self.outlier_timestamps],
            "profile_correlation": round2(self.profile_correlation),
            "related_event_ids": list(self.related_event_ids),
            "signals": list(self.signals),
        }


@dataclass(frozen=True)
class Anomaly:
    meter_id: str
    type: str
    severity: str
    confidence: float
    evidence: Evidence
    priority: int = 0

    @property
    def anomaly(self) -> bool:
        return self.type != FALSE_POSITIVE

    def to_dict(self) -> dict:
        return {
            "meter_id": self.meter_id,
            "anomaly": self.anomaly,
            "type": self.type,
            "severity": self.severity,
            "confidence": round2(self.confidence),
            "priority": self.priority,
            "evidence": self.evidence.to_dict(),
        }

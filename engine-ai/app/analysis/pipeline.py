"""Runs the analysis steps over all meters, yielding progress events and the result."""

import logging
import time
from collections.abc import Iterator
from dataclasses import replace

import pandas as pd

from app.analysis import thresholds as t
from app.analysis.baseline import Reference, build_reference, daily_metrics, pre_change_window
from app.analysis.classify import Classification, Findings, classify, prioritize
from app.analysis.detectors import (
    PersistentChange,
    detect_data_quality,
    detect_electrical_change,
    detect_outliers,
    detect_persistent_change,
    detect_transient,
    profile_correlation,
)
from app.analysis.events import match_onset, match_outage_window
from app.analysis.models import Anomaly, Evidence, MeterMetrics

logger = logging.getLogger(__name__)

READING_COLUMNS = ["consumption_kwh", "voltage_v", "current_a", "power_factor"]


class _StepClock:
    """Builds step events and logs how long each step took."""

    def __init__(self, analysis_id: int | None) -> None:
        self._analysis_id = analysis_id
        self._step: str | None = None
        self._started = 0.0

    def start(self, step: str) -> dict:
        self.finish()
        self._step, self._started = step, time.perf_counter()
        return {"type": "step", "step": step}

    def finish(self) -> None:
        if self._step is not None:
            elapsed = (time.perf_counter() - self._started) * 1000
            logger.info(
                "step finished",
                extra={
                    "analysis_id": self._analysis_id,
                    "step": self._step,
                    "duration_ms": round(elapsed, 1),
                },
            )
        self._step = None


def prepare_series(
    readings: pd.DataFrame, analysis_id: int | None = None
) -> dict[str, pd.DataFrame]:
    """Per-meter hourly series sorted by timestamp; meters under MIN_HOURS are skipped."""
    series: dict[str, pd.DataFrame] = {}
    for meter_id, rows in readings.groupby("meter_id", sort=True):
        clean = (
            rows.dropna(subset=READING_COLUMNS)
            .drop_duplicates("timestamp")
            .sort_values("timestamp")
            .reset_index(drop=True)
        )
        if len(clean) < t.MIN_HOURS:
            logger.warning(
                "meter skipped: not enough readings",
                extra={"analysis_id": analysis_id, "meter_id": meter_id, "hours": len(clean)},
            )
            continue
        frame = clean[["timestamp"]].copy()
        frame[READING_COLUMNS] = clean[READING_COLUMNS].astype(float)
        series[str(meter_id)] = frame
    return series


def establish_reference(series: pd.DataFrame) -> tuple[Reference, PersistentChange | None]:
    """Two-pass baseline: full series first, then the full days before a persistent change."""
    everything = pd.Series(True, index=series.index)
    first = build_reference(series, everything, reliable=True)
    change = detect_persistent_change(series, first)
    if change is None:
        return first, None
    window = pre_change_window(series, change.start)
    if window is None:
        return replace(first, reliable=False), change
    second = build_reference(series, window, reliable=True)
    return second, detect_persistent_change(series, second)


def _evidence(
    metrics: MeterMetrics,
    ref: Reference,
    findings: Findings,
    result: Classification,
    outliers: list[pd.Timestamp],
    correlation: float | None,
) -> Evidence:
    change, transient, dq = findings.change, findings.transient, findings.data_quality
    related: set[int] = set()
    if change:
        related.update(e.id for e in findings.change_events)
    if transient:
        related.update(e.id for e in findings.transient_events)
    if dq:
        related.update(e.id for e in findings.data_quality_events)
    return Evidence(
        baseline_kwh=metrics.baseline_kwh,
        current_kwh=metrics.current_kwh,
        variation_pct=metrics.variation_pct,
        change_start=metrics.change_start,
        baseline_reliable=ref.reliable,
        shift_pct=change.shift_pct * 100 if change else None,
        effect_size=change.effect if change else None,
        changed_vars=findings.electrical.changed_vars if findings.electrical else None,
        transient=(
            {
                "start": transient.start,
                "end": transient.end,
                "hours": transient.hours,
                "mean_deviation_pct": transient.mean_deviation * 100,
                "effect_size": transient.effect,
            }
            if transient
            else None
        ),
        failed_checks=(
            [{"check": name, "hours": hours} for name, hours in dq.failed_checks.items()]
            if dq
            else []
        ),
        outlier_timestamps=[ts.to_pydatetime() for ts in outliers],
        profile_correlation=correlation,
        related_event_ids=sorted(related),
        signals=list(result.signals),
    )


def run_analysis(
    readings: pd.DataFrame,
    events: pd.DataFrame,
    analysis_id: int | None = None,
) -> Iterator[dict]:
    """Yield one step event per pipeline step, then the result event.

    analysis_id is only attached to log lines; it never changes the computation.
    """
    clock = _StepClock(analysis_id)

    yield clock.start("READINGS")
    series = prepare_series(readings, analysis_id)

    yield clock.start("BASELINE")
    refs: dict[str, Reference] = {}
    changes: dict[str, PersistentChange | None] = {}
    metrics: dict[str, MeterMetrics] = {}
    for meter_id, s in series.items():
        ref, change = establish_reference(s)
        if ref.baseline_kwh == 0:
            logger.warning(
                "meter skipped: no baseline (zero or no full day)",
                extra={"analysis_id": analysis_id, "meter_id": meter_id},
            )
            continue
        refs[meter_id], changes[meter_id] = ref, change
        metrics[meter_id] = daily_metrics(meter_id, s, ref, change.start if change else None)

    yield clock.start("DETECTION")
    transients, outliers, correlations = {}, {}, {}
    for meter_id, ref in refs.items():
        s = series[meter_id]
        transients[meter_id] = detect_transient(s, ref)
        outliers[meter_id] = detect_outliers(s, ref)
        correlations[meter_id] = profile_correlation(s, ref)

    yield clock.start("CORRELATION")
    electrical, quality = {}, {}
    for meter_id, ref in refs.items():
        s, change = series[meter_id], changes[meter_id]
        electrical[meter_id] = detect_electrical_change(s, ref, change) if change else None
        quality[meter_id] = detect_data_quality(s, ref)

    yield clock.start("EVENTS")
    anomalies = []
    for meter_id, ref in refs.items():
        own = events[events["meter_id"] == meter_id]
        change, transient, dq = changes[meter_id], transients[meter_id], quality[meter_id]
        findings = Findings(
            change=change,
            transient=transient,
            electrical=electrical[meter_id],
            data_quality=dq,
            change_events=match_onset(own, change.start) if change else (),
            transient_events=(
                match_outage_window(own, transient.start, transient.end) if transient else ()
            ),
            data_quality_events=match_onset(own, dq.onset) if dq else (),
            baseline_reliable=ref.reliable,
        )
        result = classify(findings)
        if result is None:
            continue
        evidence = _evidence(
            metrics[meter_id], ref, findings, result, outliers[meter_id], correlations[meter_id]
        )
        anomalies.append(
            Anomaly(
                meter_id=meter_id,
                type=result.type,
                severity=result.severity,
                confidence=result.confidence,
                evidence=evidence,
            )
        )
    ranked = prioritize(anomalies)
    clock.finish()

    yield {
        "type": "result",
        "metrics": [m.to_dict() for m in metrics.values()],
        "anomalies": [a.to_dict() for a in ranked],
    }

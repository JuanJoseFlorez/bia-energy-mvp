from datetime import datetime

import pandas as pd
import pytest

from app.analysis.classify import Findings, classify, confidence, prioritize
from app.analysis.detectors import DataQuality, ElectricalChange, PersistentChange, Transient
from app.analysis.events import EventRef
from app.analysis.models import Anomaly, Evidence

TS = pd.Timestamp("2026-09-10 00:00")


def change(shift: float = 0.5, effect: float = 20.0) -> PersistentChange:
    return PersistentChange(start=TS, shift_pct=shift, effect=effect, post_hours=96)


def electrical(fired: bool, signals: tuple[str, ...] = ()) -> ElectricalChange:
    return ElectricalChange(
        changed_vars={}, power_factor_drop=0.0, power_ratio_shift=0.0, fired=fired, signals=signals
    )


TRANSIENT = Transient(start=TS, end=TS, hours=12, mean_deviation=-0.8, effect=-20.0)
DQ = DataQuality(onset=TS, flagged_hours=12, failed_checks={"voltage_jump": 5, "flatline": 7})
OP = (EventRef(1, "OPERATIONAL_CHANGE"),)
OUTAGE = (EventRef(2, "SCHEDULED_OUTAGE"),)
UNKNOWN = (EventRef(3, "UNKNOWN"),)
DQ_EVENT = (EventRef(4, "DATA_QUALITY"),)


@pytest.mark.parametrize(
    ("name", "findings", "expected"),
    [
        (
            "rule 1: data quality without change",
            Findings(data_quality=DQ, data_quality_events=DQ_EVENT),
            ("DATA_QUALITY", "HIGH", ("flatline", "data_quality_event")),
        ),
        (
            "rule 2: change explained by operational event",
            Findings(change=change(), electrical=electrical(False), change_events=OP),
            ("EXPLAINABLE_ANOMALY", "MEDIUM", ("explained_by_event",)),
        ),
        (
            "rule 2 wins over data quality",
            Findings(
                change=change(), electrical=electrical(False), data_quality=DQ, change_events=OP
            ),
            ("EXPLAINABLE_ANOMALY", "MEDIUM", ("explained_by_event",)),
        ),
        (
            "rule 3: unexplained change >= 100 % is HIGH",
            Findings(change=change(shift=1.2), electrical=electrical(False)),
            ("REAL_ANOMALY", "HIGH", ("no_explaining_event",)),
        ),
        (
            "rule 3: unexplained change with electrical change is HIGH",
            Findings(
                change=change(shift=0.4),
                electrical=electrical(True, ("power_factor_drop",)),
                change_events=UNKNOWN,
            ),
            ("REAL_ANOMALY", "HIGH", ("no_explaining_event", "power_factor_drop")),
        ),
        (
            "rule 3: moderate unexplained change is MEDIUM",
            Findings(change=change(shift=0.4), electrical=electrical(False)),
            ("REAL_ANOMALY", "MEDIUM", ("no_explaining_event",)),
        ),
        (
            "rule 3: outage event does not explain a persistent change",
            Findings(change=change(shift=0.4), electrical=electrical(False), change_events=OUTAGE),
            ("REAL_ANOMALY", "MEDIUM", ("no_explaining_event",)),
        ),
        (
            "rule 4: transient inside scheduled outage",
            Findings(transient=TRANSIENT, transient_events=OUTAGE),
            ("FALSE_POSITIVE", "LOW", ("explained_by_event",)),
        ),
        (
            "rule 5: unexplained transient",
            Findings(transient=TRANSIENT, transient_events=UNKNOWN),
            ("REAL_ANOMALY", "MEDIUM", ()),
        ),
    ],
)
def test_classification_rules(name, findings, expected):
    result = classify(findings)
    assert (result.type, result.severity, result.signals) == expected, name


def test_rule_6_no_finding_is_normal():
    assert classify(Findings()) is None


@pytest.mark.parametrize(
    ("strength", "signals", "reliable", "expected"),
    [
        (0.0, (), True, 0.50),
        (1.0, (), True, 0.80),
        (0.5, ("a", "b"), True, 0.75),
        (1.0, ("a", "b", "c", "d"), True, 0.99),  # capped
        (1.0, ("a",), False, 0.70),  # unreliable baseline penalty
    ],
)
def test_confidence_formula(strength, signals, reliable, expected):
    assert confidence(strength, signals, reliable) == expected


def test_confidence_uses_effect_strength():
    weak = classify(Findings(change=change(effect=10.0), electrical=electrical(False)))
    strong = classify(Findings(change=change(effect=40.0), electrical=electrical(False)))
    assert weak.confidence == 0.70  # 0.5 + 0.3 * 0.5 + 0.05 * 1
    assert strong.confidence == 0.85  # strength capped at 1


def anomaly(meter_id: str, kind: str, severity: str, conf: float, variation: float) -> Anomaly:
    evidence = Evidence(
        baseline_kwh=1.0,
        current_kwh=1.0,
        variation_pct=variation,
        change_start=datetime(2026, 9, 10),
        baseline_reliable=True,
    )
    return Anomaly(meter_id, kind, severity, conf, evidence)


def test_prioritize_order_and_tie_breaks():
    ranked = prioritize(
        [
            anomaly("X-8", "FALSE_POSITIVE", "LOW", 0.99, 90),
            anomaly("X-7", "EXPLAINABLE_ANOMALY", "MEDIUM", 0.90, 50),
            anomaly("X-6", "DATA_QUALITY", "HIGH", 0.99, 1),
            anomaly("X-5", "REAL_ANOMALY", "MEDIUM", 0.99, 300),
            anomaly("X-4", "REAL_ANOMALY", "HIGH", 0.80, 10),
            anomaly("X-3", "REAL_ANOMALY", "HIGH", 0.90, 10),
            anomaly("X-2", "REAL_ANOMALY", "HIGH", 0.90, -20),
            anomaly("X-1", "REAL_ANOMALY", "HIGH", 0.90, 20),
        ]
    )
    assert [a.meter_id for a in ranked] == [
        "X-1",  # same type/severity/confidence/|variation| as X-2 -> meter_id asc
        "X-2",
        "X-3",  # lower |variation|
        "X-4",  # lower confidence
        "X-5",  # MEDIUM after HIGH
        "X-6",  # DATA_QUALITY after REAL_ANOMALY
        "X-7",
        "X-8",
    ]
    assert [a.priority for a in ranked] == list(range(1, 9))

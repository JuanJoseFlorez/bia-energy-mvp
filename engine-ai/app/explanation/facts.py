"""Spanish facts about one anomaly, built only from its evidence and related events.

The same facts feed the templates and the LLM prompt. The prompt text also defines which
numbers, dates and times the LLM may use (see tokens and review.py).
"""

import re
from dataclasses import dataclass
from datetime import datetime
from decimal import ROUND_HALF_UP, Decimal

import pandas as pd

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

MONTHS = ("ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic")

ANOMALY_LABELS = {
    REAL_ANOMALY: "anomalía real",
    DATA_QUALITY: "calidad de datos",
    EXPLAINABLE_ANOMALY: "anomalía explicable",
    FALSE_POSITIVE: "falso positivo",
}
SEVERITY_LABELS = {HIGH: "alta", MEDIUM: "media", LOW: "baja"}
EVENT_LABELS = {
    "OPERATIONAL_CHANGE": "cambio operativo",
    "SCHEDULED_OUTAGE": "parada programada",
    "DATA_QUALITY": "reporte de calidad de datos",
    "UNKNOWN": "sin evento operativo conocido",
}
OTHER_EVENT_LABEL = "evento no clasificado"
CHECK_LABELS = {
    "voltage_out_of_range": "voltaje fuera de rango",
    "voltage_jump": "saltos de voltaje",
    "current_jump_flat_consumption": "saltos de corriente con consumo plano",
    "power_ratio_inconsistent": "kWh inconsistente con V·I·PF",
    "power_factor_jump": "saltos de factor de potencia",
    "power_factor_out_of_range": "factor de potencia fuera de [0, 1]",
    "flatline": "valores repetidos",
}
# evidence.changed_vars name -> (Spanish label, decimals, unit suffix), in output order
VARIABLES = {
    "power_factor": ("factor de potencia", 2, ""),
    "current_a": ("corriente", 1, " A"),
    "voltage_v": ("voltaje", 1, " V"),
}
NO_EVENTS = "ningún evento registrado"

DATE_RE = re.compile(r"(?<!\d)(\d{1,2})-(" + "|".join(MONTHS) + r")(?![a-zñáéíóú])", re.I)
TIME_RE = re.compile(r"(?<!\d)(\d{1,2}):(\d{2})(?!\d)")
# Optional minus (not glued to a word: "X-12" is 12), digits with . and , inside, optional %
NUMBER_RE = re.compile(r"(?:(?<!\w)([-−]))?(\d(?:[\d.,]*\d)?)(\s?%)?")
SPANISH_NUMBER_RE = re.compile(r"\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+(?:,\d+)?")


def number(value: float, decimals: int = 1) -> str:
    """Spanish number with half-up rounding: thousands dot, decimal comma (1.052,7)."""
    step = Decimal(1).scaleb(-decimals)
    rounded = Decimal(repr(float(value))).quantize(step, rounding=ROUND_HALF_UP)
    if rounded == 0:
        rounded = abs(rounded)  # never "-0,0"
    text = f"{rounded:,.{decimals}f}"
    return text.replace(",", "_").replace(".", ",").replace("_", ".")


def percent(value: float) -> str:
    """Signed percentage with one decimal and a space: +109,7 %, -79,7 %."""
    text = number(value)
    return f"{text if text.startswith('-') else '+' + text} %"


def moment(value: datetime) -> str:
    """UTC date and time with a Spanish month abbreviation: 12-sep 14:00."""
    return f"{value.day:02d}-{MONTHS[value.month - 1]} {value.hour:02d}:{value.minute:02d}"


def event_label(event_type: str) -> str:
    return EVENT_LABELS.get(event_type, OTHER_EVENT_LABEL)


@dataclass(frozen=True)
class EventFact:
    moment: str
    label: str
    description: str  # original free text (English); only the LLM prompt shows it


@dataclass(frozen=True)
class TransientFact:
    start: str
    end: str
    hours: str
    deviation: str


@dataclass(frozen=True)
class Facts:
    meter_id: str
    type: str
    type_label: str
    severity_label: str
    confidence: str
    baseline: str
    current: str
    variation: str
    change_start: str | None
    changed_vars: tuple[tuple[str, str], ...]  # (label, "0,94 → 0,74")
    power_factor_drop: bool
    transient: TransientFact | None
    failed_checks: tuple[tuple[str, str], ...]  # (label, "16 h")
    events: tuple[EventFact, ...]

    def lines(self) -> list[tuple[str, str]]:
        """Ordered (label, text) pairs: the HECHOS list of the prompt."""
        out = [
            ("tipo", self.type_label),
            ("severidad", self.severity_label),
            ("confianza", self.confidence),
            ("baseline diario", self.baseline),
            ("consumo último día", self.current),
            ("variación", self.variation),
        ]
        if self.change_start:
            out.append(("inicio del cambio", self.change_start))
        out.extend(self.changed_vars)
        if self.transient:
            t = self.transient
            out.append(
                ("transitorio", f"{t.start} a {t.end}, {t.hours}, desviación media {t.deviation}")
            )
        out.extend(("chequeo fallido", f"{label}, {hours}") for label, hours in self.failed_checks)
        out.extend(("evento relacionado", f"{e.label}, {e.moment}") for e in self.events)
        if not self.events:
            out.append(("eventos relacionados", NO_EVENTS))
        return out


def _changed_vars(changed: dict[str, dict[str, float]] | None) -> tuple[tuple[str, str], ...]:
    if not changed:
        return ()
    out = []
    for name, (label, decimals, unit) in VARIABLES.items():
        if name in changed:
            values = changed[name]
            before, after = number(values["before"], decimals), number(values["after"], decimals)
            out.append((label, f"{before} → {after}{unit}"))
    return tuple(out)


def _events(events: pd.DataFrame, ids: list[int]) -> tuple[EventFact, ...]:
    related = events[events["id"].isin(ids)].sort_values("id")
    out = []
    for row in related.to_dict("records"):
        description = row.get("description")
        out.append(
            EventFact(
                moment=moment(row["event_timestamp"]),
                label=event_label(str(row["event_type"])),
                description=description.strip() if isinstance(description, str) else "",
            )
        )
    return tuple(out)


def build_facts(anomaly: Anomaly, events: pd.DataFrame) -> Facts:
    """Facts for one anomaly; events is the whole events table (filtered by evidence ids)."""
    ev = anomaly.evidence
    transient = None
    if ev.transient is not None:
        transient = TransientFact(
            start=moment(ev.transient["start"]),
            end=moment(ev.transient["end"]),
            hours=f"{ev.transient['hours']} h",
            deviation=percent(ev.transient["mean_deviation_pct"]),
        )
    return Facts(
        meter_id=anomaly.meter_id,
        type=anomaly.type,
        type_label=ANOMALY_LABELS[anomaly.type],
        severity_label=SEVERITY_LABELS[anomaly.severity],
        confidence=number(anomaly.confidence, 2),
        baseline=f"{number(ev.baseline_kwh)} kWh",
        current=f"{number(ev.current_kwh)} kWh",
        variation=percent(ev.variation_pct),
        change_start=moment(ev.change_start) if ev.change_start else None,
        changed_vars=_changed_vars(ev.changed_vars),
        power_factor_drop="power_factor_drop" in ev.signals,
        transient=transient,
        failed_checks=tuple(
            (CHECK_LABELS.get(c["check"], c["check"]), f"{c['hours']} h") for c in ev.failed_checks
        ),
        events=_events(events, ev.related_event_ids),
    )


def render_prompt(facts: Facts, base_action: str) -> str:
    """User message for the LLM: everything it may cite, and nothing else."""
    lines = [f"Medidor: {facts.meter_id}", f"Acción base: {base_action}", "HECHOS:"]
    lines.extend(f"- {label}: {text}" for label, text in facts.lines())
    lines.append("EVENTOS:")
    for e in facts.events:
        parts = [e.moment, e.label] + ([e.description] if e.description else [])
        lines.append("- " + " · ".join(parts))
    if not facts.events:
        lines.append("- ninguno")
    return "\n".join(lines)


@dataclass(frozen=True)
class Tokens:
    dates: frozenset[str]  # "12-sep"
    times: frozenset[str]  # "14:00"
    # (value, is_percent, written decimal digits); value None = unparseable
    numbers: tuple[tuple[float | None, bool, int], ...]


def _decimals(digits: str) -> int:
    """Digits written after the decimal comma; thousands dots ("1.053") are not decimals."""
    return len(digits.partition(",")[2])


def _parse(sign: str, digits: str) -> tuple[float | None, int]:
    if not SPANISH_NUMBER_RE.fullmatch(digits):
        return None, 0
    value = float(digits.replace(".", "").replace(",", "."))
    return (-value if sign else value), _decimals(digits)


def _number_token(sign: str, digits: str, pct: str) -> tuple[float | None, bool, int]:
    value, decimals = _parse(sign, digits)
    return value, bool(pct), decimals


def tokens(text: str) -> Tokens:
    """Dates, then times, then every remaining number (Spanish format) in the text."""
    dates = frozenset(f"{int(day):02d}-{month.lower()}" for day, month in DATE_RE.findall(text))
    rest = DATE_RE.sub(" ", text)
    times = frozenset(f"{int(hour):02d}:{minute}" for hour, minute in TIME_RE.findall(rest))
    rest = TIME_RE.sub(" ", rest)
    numbers = tuple(_number_token(*match) for match in NUMBER_RE.findall(rest))
    return Tokens(dates=dates, times=times, numbers=numbers)

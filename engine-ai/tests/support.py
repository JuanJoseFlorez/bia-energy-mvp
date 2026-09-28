"""Test data: the real seed CSVs and synthetic meter series."""

from pathlib import Path

import numpy as np
import pandas as pd

from app.analysis.models import Anomaly, Evidence

DB_INIT = Path(__file__).resolve().parents[2] / "db-init"


def load_readings_csv() -> pd.DataFrame:
    df = pd.read_csv(DB_INIT / "readings.csv", parse_dates=["timestamp"])
    return df.drop(columns=["status"])


def load_events_csv() -> pd.DataFrame:
    df = pd.read_csv(DB_INIT / "events.csv", parse_dates=["event_timestamp"])
    # ids follow the COPY order into the SERIAL column
    df.insert(0, "id", range(1, len(df) + 1))
    return df[["id", "meter_id", "event_timestamp", "event_type", "description"]]


START = pd.Timestamp("2026-09-01 00:00")
# Daily shape like the real data: night 1.0, shoulders 1.28, working hours 1.55
SHAPE = np.array([1.0] * 6 + [1.28] * 2 + [1.55] * 10 + [1.28] * 6)


def make_series(days: int = 14, level: float = 40.0, noise: float = 0.02, seed: int = 0):
    """Synthetic hourly meter with daily shape and small multiplicative noise."""
    rng = np.random.default_rng(seed)
    n = days * 24
    ts = pd.date_range(START, periods=n, freq="h")
    shape = SHAPE[ts.hour]
    wiggle = 1 + noise * rng.standard_normal(n)
    return pd.DataFrame(
        {
            "timestamp": ts,
            "consumption_kwh": level * shape * wiggle,
            "voltage_v": 220 + 0.8 * rng.standard_normal(n),
            "current_a": 180 * shape * (1 + noise * rng.standard_normal(n)),
            "power_factor": 0.93 + 0.01 * rng.standard_normal(n),
        }
    )


def as_readings(series: pd.DataFrame, meter_id: str = "X-1") -> pd.DataFrame:
    out = series.copy()
    out.insert(0, "meter_id", meter_id)
    return out


def no_events() -> pd.DataFrame:
    return pd.DataFrame(
        {
            "id": pd.Series([], dtype=int),
            "meter_id": pd.Series([], dtype=str),
            "event_timestamp": pd.Series([], dtype="datetime64[ns]"),
            "event_type": pd.Series([], dtype=str),
            "description": pd.Series([], dtype=str),
        }
    )


def make_events(*rows: tuple) -> pd.DataFrame:
    """Events table from (id, meter_id, "YYYY-MM-DD HH:MM", event_type, description) rows."""
    frame = pd.DataFrame(
        rows, columns=["id", "meter_id", "event_timestamp", "event_type", "description"]
    )
    frame["event_timestamp"] = pd.to_datetime(frame["event_timestamp"])
    return frame


def make_anomaly(
    kind: str = "REAL_ANOMALY",
    severity: str = "HIGH",
    confidence: float = 0.9,
    meter_id: str = "X-1",
    **evidence,
) -> Anomaly:
    """Anomaly with a plain evidence; keyword arguments override Evidence fields."""
    fields = {
        "baseline_kwh": 1000.0,
        "current_kwh": 1500.0,
        "variation_pct": 50.0,
        "change_start": None,
        "baseline_reliable": True,
        **evidence,
    }
    return Anomaly(meter_id, kind, severity, confidence, Evidence(**fields))

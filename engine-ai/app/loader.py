"""Read-only data access. The only module that touches the database."""

import pandas as pd
from psycopg_pool import ConnectionPool

CONNECT_TIMEOUT_SECONDS = 2.0

READINGS_SQL = """
SELECT meter_id, timestamp,
       consumption_kwh::float8, voltage_v::float8, current_a::float8, power_factor::float8
FROM readings
ORDER BY meter_id, timestamp
"""
READINGS_COLUMNS = [
    "meter_id",
    "timestamp",
    "consumption_kwh",
    "voltage_v",
    "current_a",
    "power_factor",
]

EVENTS_SQL = """
SELECT id, meter_id, event_timestamp, event_type, description
FROM events
ORDER BY id
"""
EVENTS_COLUMNS = ["id", "meter_id", "event_timestamp", "event_type", "description"]


def create_pool(conninfo: str) -> ConnectionPool:
    """Pool that connects lazily on open(); the app opens it at startup."""
    return ConnectionPool(conninfo, min_size=1, max_size=4, open=False)


def ping(pool: ConnectionPool) -> None:
    """Raise if the database does not answer SELECT 1."""
    with pool.connection(timeout=CONNECT_TIMEOUT_SECONDS) as conn:
        conn.execute("SELECT 1")


def load_data(pool: ConnectionPool) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Readings and events as DataFrames (timestamps naive UTC)."""
    with pool.connection(timeout=CONNECT_TIMEOUT_SECONDS) as conn:
        readings = conn.execute(READINGS_SQL).fetchall()
        events = conn.execute(EVENTS_SQL).fetchall()
    readings_df = pd.DataFrame(readings, columns=READINGS_COLUMNS)
    readings_df["timestamp"] = pd.to_datetime(readings_df["timestamp"])
    events_df = pd.DataFrame(events, columns=EVENTS_COLUMNS)
    events_df["event_timestamp"] = pd.to_datetime(events_df["event_timestamp"])
    return readings_df, events_df

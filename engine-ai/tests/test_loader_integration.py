"""Loader against the compose database; run with `make test-integration`."""

import os

import pytest

from app.analysis.pipeline import run_analysis
from app.loader import create_pool, load_data, ping

URL = os.environ.get("TEST_DATABASE_URL")

pytestmark = pytest.mark.skipif(not URL, reason="TEST_DATABASE_URL not set")


@pytest.fixture(scope="module")
def pool():
    p = create_pool(URL)
    p.open(wait=True, timeout=5)
    yield p
    p.close()


def test_ping(pool):
    ping(pool)


def test_load_data(pool, real_events):
    readings, events = load_data(pool)
    assert len(readings) == 4032
    assert readings["meter_id"].nunique() == 12
    assert readings["consumption_kwh"].dtype == float
    assert str(readings["timestamp"].dtype).startswith("datetime64")
    assert len(events) == 4
    assert list(events.columns) == [
        "id",
        "meter_id",
        "event_timestamp",
        "event_type",
        "description",
    ]
    assert list(events["description"]) == list(real_events["description"])


def test_loaded_data_gives_same_result_as_csv(pool, real_readings, real_events):
    readings, events = load_data(pool)
    from_db = list(run_analysis(readings, events))[-1]
    from_csv = list(run_analysis(real_readings, real_events))[-1]
    assert from_db == from_csv

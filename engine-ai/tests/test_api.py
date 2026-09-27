import json

import pandas as pd
from fastapi.testclient import TestClient

from app.api import create_app
from tests.support import as_readings, make_series, no_events


def ok() -> None:
    return None


def down() -> None:
    raise ConnectionError("db down")


def fake_loader():
    readings = pd.concat(
        [as_readings(make_series(seed=1), "X-1"), as_readings(make_series(seed=2), "X-2")]
    )
    return readings, no_events()


def client(check_db=ok, load_data=fake_loader) -> TestClient:
    return TestClient(create_app(check_db=check_db, load_data=load_data))


def lines(response) -> list[dict]:
    return [json.loads(line) for line in response.text.splitlines()]


def test_health_ok():
    r = client().get("/health")
    assert r.status_code == 200
    assert r.json() == {"status": "ok"}


def test_health_database_down():
    r = client(check_db=down).get("/health")
    assert r.status_code == 503
    assert r.json() == {"error": "database unavailable"}


def test_analyze_streams_steps_then_result():
    r = client().post("/analyze", json={"analysis_id": 7})
    assert r.status_code == 200
    assert r.headers["content-type"].startswith("application/x-ndjson")
    out = lines(r)
    assert [e.get("step") for e in out[:5]] == [
        "READINGS",
        "BASELINE",
        "DETECTION",
        "CORRELATION",
        "EVENTS",
    ]
    assert all(e["type"] == "step" for e in out[:5])
    assert out[5]["type"] == "result"
    assert len(out) == 6
    assert [m["meter_id"] for m in out[5]["metrics"]] == ["X-1", "X-2"]
    assert out[5]["anomalies"] == []


def test_analyze_without_body():
    r = client().post("/analyze")
    assert r.status_code == 200
    assert lines(r)[-1]["type"] == "result"


def test_analyze_rejects_non_integer_analysis_id():
    r = client().post("/analyze", json={"analysis_id": "seven"})
    assert r.status_code == 422


def test_analyze_load_failure_returns_503():
    def failing_loader():
        raise ConnectionError("db down")

    r = client(load_data=failing_loader).post("/analyze")
    assert r.status_code == 503
    assert r.json() == {"error": "database unavailable"}


def test_analyze_error_line_when_analysis_raises():
    def broken_loader():
        # Missing reading columns make the pipeline raise after the first step.
        return pd.DataFrame({"meter_id": ["X-1"], "timestamp": [pd.Timestamp("2026-09-01")]}), (
            no_events()
        )

    r = client(load_data=broken_loader).post("/analyze")
    out = lines(r)
    assert out[0] == {"type": "step", "step": "READINGS"}
    assert out[-1] == {"type": "error", "message": "analysis failed"}
    assert all(e["type"] != "result" for e in out)

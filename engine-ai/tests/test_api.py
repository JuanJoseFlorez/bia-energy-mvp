import json

import pandas as pd
from fastapi.testclient import TestClient

from app.api import create_app, create_lifespan
from app.explanation.review import Draft
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


def client(check_db=ok, load_data=fake_loader, drafter=None) -> TestClient:
    return TestClient(create_app(check_db=check_db, load_data=load_data, drafter=drafter))


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
    assert [e.get("step") for e in out[:7]] == [
        "READINGS",
        "BASELINE",
        "DETECTION",
        "CORRELATION",
        "EVENTS",
        "EXPLANATION",
        "RECOMMENDATION",
    ]
    assert all(e["type"] == "step" for e in out[:7])
    assert out[7]["type"] == "result"
    assert len(out) == 8
    assert [m["meter_id"] for m in out[7]["metrics"]] == ["X-1", "X-2"]
    assert out[7]["anomalies"] == []


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


def test_analyze_passes_the_drafter_to_the_analysis():
    def stepped_loader():
        s = make_series()
        s.loc[s["timestamp"] >= "2026-09-10", ["consumption_kwh", "current_a"]] *= 1.8
        return as_readings(s, "X-1"), no_events()

    def drafter(requests):
        data = {"reason": "Motivo.", "explanation": "Detalle.", "action_detail": "Paso."}
        return {r.meter_id: Draft(data=data) for r in requests}

    out = lines(client(load_data=stepped_loader, drafter=drafter).post("/analyze"))
    [anomaly] = out[-1]["anomalies"]
    assert anomaly["explanation_source"] == "llm"
    assert anomaly["recommended_action"] == "Investigar medidor e instalación. Paso."


class FakeDbPool:
    def __init__(self):
        self.calls = []

    def open(self):
        self.calls.append("open")

    def close(self):
        self.calls.append("close")


class FakeLlmPool:
    def __init__(self):
        self.shutdowns = []

    def shutdown(self, **kwargs):
        self.shutdowns.append(kwargs)


def test_lifespan_opens_db_and_shuts_llm_pool_down():
    db, llm = FakeDbPool(), FakeLlmPool()
    app = create_app(check_db=ok, load_data=fake_loader, lifespan=create_lifespan(db, llm))
    with TestClient(app) as c:
        assert c.get("/health").status_code == 200
        assert db.calls == ["open"]
        assert llm.shutdowns == []
    assert db.calls == ["open", "close"]
    assert llm.shutdowns == [{"wait": False, "cancel_futures": True}]


def test_lifespan_without_llm_pool():
    db = FakeDbPool()
    app = create_app(check_db=ok, load_data=fake_loader, lifespan=create_lifespan(db, None))
    with TestClient(app):
        pass
    assert db.calls == ["open", "close"]

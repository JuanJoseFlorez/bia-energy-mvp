"""Real provider smoke test on the seed data; run with `make test-llm` (never in `make test`)."""

import logging
import os

import pytest

from app.analysis.pipeline import run_analysis
from app.config import load_settings
from app.explanation.llm import create_llm_pool, make_drafter

PLACEHOLDER_DB = {"DB_HOST": "-", "DB_USER": "-", "DB_PASSWORD": "-", "DB_NAME": "-"}
SETTINGS = load_settings({**PLACEHOLDER_DB, **os.environ})

pytestmark = pytest.mark.skipif(
    os.environ.get("LLM_LIVE") != "1" or not SETTINGS.llm_enabled,
    reason="live LLM test: run `make test-llm` with LLM_MODEL and LLM_API_KEY set",
)


def test_seed_anomalies_get_llm_texts(real_readings, real_events, caplog):
    caplog.set_level(logging.INFO)
    pool = create_llm_pool(SETTINGS)
    drafter = make_drafter(
        pool, SETTINGS.llm_model, SETTINGS.llm_api_key, SETTINGS.llm_timeout_seconds
    )
    try:
        result = list(run_analysis(real_readings, real_events, drafter=drafter))[-1]
    finally:
        pool.shutdown(wait=False, cancel_futures=True)
    rejections = {r.meter_id: r.reason for r in caplog.records if r.msg == "llm_fallback"}
    attempts = {r.meter_id: r.attempts for r in caplog.records if r.msg == "anomaly explained"}
    retries = [(r.meter_id, r.attempt, r.reason) for r in caplog.records if r.msg == "llm_retry"]
    print(f"\nmodel: {SETTINGS.llm_model}; retries: {retries}")
    for a in result["anomalies"]:
        meter_id = a["meter_id"]
        print(
            f"{meter_id} {a['explanation_source']} attempts={attempts.get(meter_id)} "
            f"{rejections.get(meter_id, '')}"
        )
        print(f"  reason: {a['reason']}")
        print(f"  explanation: {a['explanation']}")
        print(f"  action: {a['recommended_action']}")
    leaked = any(SETTINGS.llm_api_key in str(r.__dict__) for r in caplog.records)
    assert not leaked
    assert any(a["explanation_source"] == "llm" for a in result["anomalies"])

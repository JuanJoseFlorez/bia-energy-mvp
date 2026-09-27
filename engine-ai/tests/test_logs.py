import json
import logging

from app.logs import JsonFormatter


def test_json_formatter_includes_extra_fields():
    record = logging.makeLogRecord(
        {"name": "engine", "levelname": "INFO", "msg": "step finished", "analysis_id": 7}
    )
    entry = json.loads(JsonFormatter().format(record))
    assert entry["msg"] == "step finished"
    assert entry["level"] == "INFO"
    assert entry["analysis_id"] == 7
    assert "time" in entry

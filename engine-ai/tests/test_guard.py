import re
from pathlib import Path

APP = Path(__file__).resolve().parents[1] / "app"


def test_no_meter_ids_in_app_code():
    offenders = [
        str(path.relative_to(APP))
        for path in APP.rglob("*.py")
        if re.search(r"M-\d{3}", path.read_text())
    ]
    assert offenders == []

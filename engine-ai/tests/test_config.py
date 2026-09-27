import pytest

from app.config import load_settings

BASE = {"DB_HOST": "db", "DB_USER": "bia", "DB_PASSWORD": "secret", "DB_NAME": "bia_energy"}


def test_load_settings_defaults():
    s = load_settings(BASE)
    assert (s.db_host, s.db_port, s.db_user, s.db_name) == ("db", 5432, "bia", "bia_energy")
    assert s.log_level == "info"


def test_load_settings_overrides():
    s = load_settings({**BASE, "DB_PORT": "6543", "LOG_LEVEL": "DEBUG"})
    assert s.db_port == 6543
    assert s.log_level == "debug"


def test_load_settings_missing_variables():
    with pytest.raises(ValueError, match="DB_HOST, DB_PASSWORD"):
        load_settings({"DB_USER": "bia", "DB_NAME": "x"})


def test_load_settings_invalid_port():
    with pytest.raises(ValueError, match="DB_PORT"):
        load_settings({**BASE, "DB_PORT": "abc"})


def test_conninfo_contains_all_parts():
    info = load_settings({**BASE, "DB_PORT": "5433"}).conninfo
    for part in ("host=db", "port=5433", "user=bia", "password=secret", "dbname=bia_energy"):
        assert part in info

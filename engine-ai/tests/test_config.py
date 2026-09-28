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


def test_llm_settings_defaults_disable_the_llm():
    s = load_settings(BASE)
    assert (s.llm_model, s.llm_api_key) == ("", "")
    assert s.llm_max_concurrency == 4
    assert s.llm_timeout_seconds == 20.0
    assert s.llm_enabled is False


def test_llm_needs_both_model_and_key():
    assert load_settings({**BASE, "LLM_MODEL": "gemini/x"}).llm_enabled is False
    assert load_settings({**BASE, "LLM_API_KEY": "k"}).llm_enabled is False
    s = load_settings({**BASE, "LLM_MODEL": "gemini/x", "LLM_API_KEY": "k"})
    assert s.llm_enabled is True


def test_llm_limits_overrides():
    s = load_settings({**BASE, "LLM_MAX_CONCURRENCY": "2", "LLM_TIMEOUT_SECONDS": "2.5"})
    assert (s.llm_max_concurrency, s.llm_timeout_seconds) == (2, 2.5)


@pytest.mark.parametrize("value", ["0", "-1", "two", "1.5"])
def test_invalid_llm_max_concurrency(value):
    with pytest.raises(ValueError, match="LLM_MAX_CONCURRENCY must be an integer >= 1"):
        load_settings({**BASE, "LLM_MAX_CONCURRENCY": value})


@pytest.mark.parametrize("value", ["0", "-3", "soon", "nan", "inf"])
def test_invalid_llm_timeout(value):
    with pytest.raises(ValueError, match="LLM_TIMEOUT_SECONDS must be a number > 0"):
        load_settings({**BASE, "LLM_TIMEOUT_SECONDS": value})


def test_secrets_are_not_in_repr():
    s = load_settings({**BASE, "LLM_MODEL": "gemini/x", "LLM_API_KEY": "top-secret-key"})
    assert "top-secret-key" not in repr(s)
    assert "secret" not in repr(s)

"""Service configuration from environment variables."""

import os
from collections.abc import Mapping
from dataclasses import dataclass, field

from psycopg.conninfo import make_conninfo

REQUIRED = ("DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME")


@dataclass(frozen=True)
class Settings:
    db_host: str
    db_port: int
    db_user: str
    db_password: str = field(repr=False)
    db_name: str
    log_level: str
    llm_model: str
    llm_api_key: str = field(repr=False)
    llm_max_concurrency: int
    llm_timeout_seconds: float

    @property
    def conninfo(self) -> str:
        return make_conninfo(
            host=self.db_host,
            port=self.db_port,
            user=self.db_user,
            password=self.db_password,
            dbname=self.db_name,
        )

    @property
    def llm_enabled(self) -> bool:
        """The LLM is used only when both the model and the API key are set."""
        return bool(self.llm_model and self.llm_api_key)


def _positive_int(env: Mapping[str, str], name: str, default: str) -> int:
    value = env.get(name) or default
    if not value.isdigit() or int(value) < 1:
        raise ValueError(f"{name} must be an integer >= 1")
    return int(value)


def _positive_float(env: Mapping[str, str], name: str, default: str) -> float:
    value = env.get(name) or default
    try:
        number = float(value)
    except ValueError:
        number = 0.0
    if not 0 < number < float("inf"):
        raise ValueError(f"{name} must be a number > 0")
    return number


def load_settings(env: Mapping[str, str] = os.environ) -> Settings:
    """Read settings; raises ValueError naming every missing required variable."""
    missing = [name for name in REQUIRED if not env.get(name)]
    if missing:
        raise ValueError(f"missing required environment variables: {', '.join(missing)}")
    port = env.get("DB_PORT") or "5432"
    if not port.isdigit():
        raise ValueError("DB_PORT must be a number")
    return Settings(
        db_host=env["DB_HOST"],
        db_port=int(port),
        db_user=env["DB_USER"],
        db_password=env["DB_PASSWORD"],
        db_name=env["DB_NAME"],
        log_level=(env.get("LOG_LEVEL") or "info").lower(),
        llm_model=(env.get("LLM_MODEL") or "").strip(),
        llm_api_key=(env.get("LLM_API_KEY") or "").strip(),
        llm_max_concurrency=_positive_int(env, "LLM_MAX_CONCURRENCY", "4"),
        llm_timeout_seconds=_positive_float(env, "LLM_TIMEOUT_SECONDS", "20"),
    )

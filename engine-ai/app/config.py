"""Service configuration from environment variables."""

import os
from collections.abc import Mapping
from dataclasses import dataclass

from psycopg.conninfo import make_conninfo

REQUIRED = ("DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME")


@dataclass(frozen=True)
class Settings:
    db_host: str
    db_port: int
    db_user: str
    db_password: str
    db_name: str
    log_level: str

    @property
    def conninfo(self) -> str:
        return make_conninfo(
            host=self.db_host,
            port=self.db_port,
            user=self.db_user,
            password=self.db_password,
            dbname=self.db_name,
        )


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
    )

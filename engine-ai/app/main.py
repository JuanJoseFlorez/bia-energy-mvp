"""Composition root: settings, logging, database pool and HTTP app."""

from contextlib import asynccontextmanager

from app.api import create_app
from app.config import load_settings
from app.loader import create_pool, load_data, ping
from app.logs import setup_logging

settings = load_settings()
setup_logging(settings.log_level)
pool = create_pool(settings.conninfo)


@asynccontextmanager
async def lifespan(_app):
    pool.open()
    yield
    pool.close()


app = create_app(
    check_db=lambda: ping(pool),
    load_data=lambda: load_data(pool),
    lifespan=lifespan,
)

"""Composition root: settings, logging, database pool, LLM pool and HTTP app."""

from app.api import create_app, create_lifespan
from app.config import load_settings
from app.explanation.llm import create_llm_pool, make_drafter
from app.loader import create_pool, load_data, ping
from app.logs import setup_logging

settings = load_settings()
setup_logging(settings.log_level)
pool = create_pool(settings.conninfo)
llm_pool = create_llm_pool(settings)
drafter = (
    make_drafter(llm_pool, settings.llm_model, settings.llm_api_key, settings.llm_timeout_seconds)
    if llm_pool is not None
    else None
)

app = create_app(
    check_db=lambda: ping(pool),
    load_data=lambda: load_data(pool),
    drafter=drafter,
    lifespan=create_lifespan(pool, llm_pool),
)

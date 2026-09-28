"""HTTP routes: GET /health and POST /analyze (NDJSON stream)."""

import json
import logging
from collections.abc import Callable, Iterator
from concurrent.futures import Executor
from contextlib import asynccontextmanager

import pandas as pd
from fastapi import FastAPI
from fastapi.responses import JSONResponse, StreamingResponse
from psycopg_pool import ConnectionPool
from pydantic import BaseModel

from app.analysis.pipeline import run_analysis
from app.explanation.review import Drafter

logger = logging.getLogger(__name__)

DbCheck = Callable[[], None]  # raises when the database is unreachable
Loader = Callable[[], tuple[pd.DataFrame, pd.DataFrame]]  # (readings, events)

UNAVAILABLE = {"error": "database unavailable"}


class AnalyzeRequest(BaseModel):
    analysis_id: int | None = None


def create_lifespan(db_pool: ConnectionPool, llm_pool: Executor | None) -> Callable:
    """Open the DB pool on startup; on shutdown close it and drop queued LLM calls."""

    @asynccontextmanager
    async def lifespan(_app):
        db_pool.open()
        yield
        db_pool.close()
        if llm_pool is not None:
            llm_pool.shutdown(wait=False, cancel_futures=True)

    return lifespan


def _line(event: dict) -> str:
    return json.dumps(event, allow_nan=False) + "\n"


def create_app(
    check_db: DbCheck,
    load_data: Loader,
    drafter: Drafter | None = None,
    lifespan: Callable | None = None,
) -> FastAPI:
    app = FastAPI(title="engine-ai", lifespan=lifespan)

    @app.get("/health")
    def health() -> JSONResponse:
        try:
            check_db()
        except Exception:
            logger.warning("health check failed", exc_info=True)
            return JSONResponse(UNAVAILABLE, status_code=503)
        return JSONResponse({"status": "ok"})

    @app.post("/analyze", response_model=None)
    def analyze_endpoint(body: AnalyzeRequest | None = None) -> StreamingResponse | JSONResponse:
        analysis_id = body.analysis_id if body else None
        context = {"analysis_id": analysis_id}
        try:
            readings, events = load_data()
        except Exception:
            logger.exception("loading data failed", extra=context)
            return JSONResponse(UNAVAILABLE, status_code=503)

        def stream() -> Iterator[str]:
            try:
                for event in run_analysis(readings, events, analysis_id, drafter):
                    yield _line(event)
            except Exception:
                logger.exception("analysis failed", extra=context)
                yield _line({"type": "error", "message": "analysis failed"})

        return StreamingResponse(stream(), media_type="application/x-ndjson")

    return app

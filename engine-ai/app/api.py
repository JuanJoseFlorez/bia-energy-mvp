"""HTTP routes: GET /health and POST /analyze (NDJSON stream)."""

import json
import logging
from collections.abc import Callable, Iterator

import pandas as pd
from fastapi import FastAPI
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel

from app.analysis.pipeline import run_analysis

logger = logging.getLogger(__name__)

DbCheck = Callable[[], None]  # raises when the database is unreachable
Loader = Callable[[], tuple[pd.DataFrame, pd.DataFrame]]  # (readings, events)

UNAVAILABLE = {"error": "database unavailable"}


class AnalyzeRequest(BaseModel):
    analysis_id: int | None = None


def _line(event: dict) -> str:
    return json.dumps(event, allow_nan=False) + "\n"


def create_app(
    check_db: DbCheck,
    load_data: Loader,
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
                for event in run_analysis(readings, events, analysis_id):
                    yield _line(event)
            except Exception:
                logger.exception("analysis failed", extra=context)
                yield _line({"type": "error", "message": "analysis failed"})

        return StreamingResponse(stream(), media_type="application/x-ndjson")

    return app

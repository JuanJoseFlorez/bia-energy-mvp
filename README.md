# BIA Energy MVP — AI Energy Management Platform

MVP to manage electric meters and use AI to **detect, explain, prioritize and recommend actions** on consumption anomalies.

> Status: **work in progress**. Database and orchestration, the backend read API and the engine-ai detection service are ready; LLM explanations, analysis endpoints and frontend pending.

## Stack

| Component    | Technology         | Responsibility                                          |
| ------------ | ------------------ | ------------------------------------------------------- |
| `backend/`   | Go                 | REST API, persistence, analysis orchestration           |
| `frontend/`  | React + TypeScript | Dashboard, meters, detail, anomalies, investigation     |
| `engine-ai/` | Python + LLM       | Baseline, anomaly detection, explanation, recommendation |
| `db`         | PostgreSQL 15      | Meters, readings, events, analysis runs, anomalies      |

## Architecture

```
Frontend (React/TS) ──HTTP──▶ Backend (Go, REST API) ──HTTP──▶ engine-ai (Python)
                                     │                               │
                                     └─────────▶ PostgreSQL ◀────────┘
```

- **Frontend talks only to backend.** `engine-ai` is internal, no published ports.
- **Detection is deterministic; the LLM explains.** Baseline, outliers, event correlation and data-quality checks use statistics/rules. The LLM writes the explanation and recommendation from the computed evidence.
- **Works without an LLM.** If `LLM_API_KEY` is empty, `engine-ai` falls back to template explanations.

## Repository layout

```
.
├── backend/            # Go REST API
├── frontend/           # React + TypeScript app (pending)
├── engine-ai/          # Python analysis engine (detection and classification)
├── db-init/
│   ├── init.sql        # Schema, meter seed, CSV load
│   ├── readings.csv    # Hourly readings
│   └── events.csv      # Known operational events
├── docker-compose.yml
└── .env.example
```

## Data

- `readings.csv` — 4,032 hourly readings: 12 meters (M-101 to M-112) × 14 days. Consumption (kWh), voltage (V), current (A), power factor.
- `events.csv` — known operational events, used to tell explainable anomalies and false positives apart.

## Data model

Created and seeded by `db-init/init.sql` on first Postgres start.

| Table           | Purpose                           | Key fields                                                                                   |
| --------------- | --------------------------------- | -------------------------------------------------------------------------------------------- |
| `meters`        | Meters                            | `meter_id` (unique), `name`, `location`, `status`                                            |
| `readings`      | Time series per meter             | `meter_id`, `timestamp`, `consumption_kwh`, `voltage_v`, `current_a`, `power_factor`         |
| `events`        | Known operational events          | `meter_id`, `event_timestamp`, `event_type`, `description`                                   |
| `analysis_runs` | One row per AI analysis execution | `status` (`PENDING`/`RUNNING`/`COMPLETED`/`FAILED`), `started_at`, `finished_at`, `summary` |
| `anomalies`     | Analysis result per meter         | `analysis_id`, `type`, `severity`, `confidence`, `priority`, `reason`, `recommended_action`, `evidence` |

- `type`: `REAL_ANOMALY`, `EXPLAINABLE_ANOMALY`, `FALSE_POSITIVE`, `DATA_QUALITY`
- `severity`: `LOW`, `MEDIUM`, `HIGH`
- `confidence`: 0 to 1
- `priority`: investigation order within a run (1 = first)

## Getting started

1. Create `.env` from the template and fill in values:

   ```bash
   cp .env.example .env
   ```

   | Variable         | Purpose                                            |
   | ---------------- | -------------------------------------------------- |
   | `DB_USER`        | Postgres user                                      |
   | `DB_PASSWORD`    | Postgres password                                  |
   | `DB_NAME`        | Database name                                      |
   | `DB_HOST_PORT`   | Host port for Postgres (default `5432`)            |
   | `LLM_MODEL`      | LLM as `provider/model` (e.g. `gemini/gemini-2.0-flash`) |
   | `LLM_API_KEY`    | LLM API key (empty = template explanations)        |
   | `VITE_API_URL`   | Backend URL used by the frontend (build time)      |

2. Start the database and the backend API:

   ```bash
   docker compose up --build db backend
   ```

   Then check the API is up:

   ```bash
   curl localhost:8080/health
   ```

> `init.sql` runs only on an empty volume. To recreate the database after a schema change: `docker compose down -v && docker compose up db` (wipes the volume; data reloads from the CSVs).

## Backend (Go)

REST API in `backend/`, organized by domain:

```
backend/
├── cmd/api/                 # entrypoint: wiring, router, graceful shutdown
└── internal/
    ├── config/              # env loading + validation
    ├── platform/            # cross-cutting: apperr, database, httpx, logger
    └── health/              # GET /health (template for new domains)
```

Each business domain is one package with `model.go`, `repository.go`, `service.go` and `handler.go` as needed. Services wrap `apperr` errors; handlers respond through `httpx`, which maps them to HTTP status codes with a single error format:

```json
{ "error": { "code": "not_found", "message": "..." }, "request_id": "..." }
```

Local development (DB in Docker, API on the host):

```bash
docker compose up -d db
cd backend
make run     # loads ../.env, connects to localhost:$DB_HOST_PORT
make test    # unit tests, no Docker needed
make lint    # gofmt + go vet
```

| Variable | Default | Purpose |
|---|---|---|
| `HTTP_PORT` | `8080` | API port |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated allowed origins |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

Endpoints:

| Method | Path | Query params | Description |
|---|---|---|---|
| `GET` | `/health` | — | `200` if the database responds, `503` otherwise |
| `GET` | `/meters` | `status` (`all`\|`ok`\|`alert`\|`critical`), `q`, `sort` (`meter_id`\|`consumption`\|`variation`\|`severity`), `order` (`asc`\|`desc`), `limit` (1–200, default 50), `offset` | Meters with period consumption, daily metrics, health and top anomaly |
| `GET` | `/meters/{meterId}` | — | Meter detail with full anomaly and events |
| `GET` | `/meters/{meterId}/readings` | `from`, `to` (RFC3339 or `YYYY-MM-DD`, inclusive) | Hourly readings, ascending |
| `GET` | `/dashboard/summary` | — | Platform KPIs |

`health`, `metrics` and `anomaly` are `null` until an analysis run has completed. Timestamps are RFC3339 UTC.

- `order` defaults to `asc` for `sort=meter_id` and `desc` for every other sort key; `sort` and `order` are case-insensitive.
- `q` matches a substring of `meter_id` only (case-insensitive; `%`, `_`, `\` are literal).
- `status=ok|alert|critical` filters on `health`, so before the first completed analysis only `status=all` returns meters.

Values in `metrics` and `anomaly` below illustrate output once an analysis has run; before that they are `null`.

Example — `GET /meters?sort=variation`:

```json
{
  "items": [{
    "id": 9, "meter_id": "M-109", "name": "Meter 109", "location": "Planta C",
    "status": "ACTIVE", "created_at": "2026-09-26T22:00:00Z",
    "period_consumption_kwh": 17526.04,
    "health": "CRITICAL",
    "metrics": { "current_kwh": 2207.60, "baseline_kwh": 1052.70, "variation_pct": 109.7, "change_start": "2026-09-12T14:00:00Z" },
    "anomaly": { "id": 3, "meter_id": "M-109", "type": "REAL_ANOMALY", "severity": "HIGH", "confidence": 0.96, "priority": 1 }
  }],
  "total": 12
}
```

Example — `GET /dashboard/summary`:

```json
{
  "meters_count": 12,
  "period": { "from": "2026-09-01T00:00:00Z", "to": "2026-09-14T23:00:00Z" },
  "total_consumption_kwh": 155250.85,
  "last_analysis": null,
  "anomalies": null
}
```

Example — `GET /meters/M-109`:

```json
{
  "id": 9, "meter_id": "M-109", "name": "Meter 109", "location": "Planta C",
  "status": "ACTIVE", "created_at": "2026-09-26T22:00:00Z",
  "period_consumption_kwh": 17526.04,
  "health": "CRITICAL",
  "metrics": { "current_kwh": 2207.60, "baseline_kwh": 1052.70, "variation_pct": 109.7, "change_start": "2026-09-12T14:00:00Z" },
  "anomaly": { "id": 3, "meter_id": "M-109", "type": "REAL_ANOMALY", "severity": "HIGH", "confidence": 0.96, "priority": 1,
               "reason": "...", "recommended_action": "...", "status": "PENDING", "detected_at": "2026-09-15T08:05:00Z" },
  "events": [{ "id": 3, "meter_id": "M-109", "timestamp": "2026-09-12T14:00:00Z", "type": "UNKNOWN", "description": "No operational event reported" }]
}
```

Example — `GET /meters/M-109/readings?from=2026-09-14&to=2026-09-14`:

```json
{
  "meter_id": "M-109",
  "items": [{ "id": 3001, "meter_id": "M-109", "timestamp": "2026-09-14T00:00:00Z", "consumption_kwh": 68.45,
              "voltage_v": 215.51, "current_a": 317.9, "power_factor": 0.719, "status": "OK" }],
  "total": 24
}
```

Integration tests run against the dockerized database:

```bash
docker compose up -d db
cd backend && make test-integration
```

## engine-ai (Python)

Deterministic analysis engine in `engine-ai/` (Python 3.12, FastAPI, pandas). It reads `readings` and `events` (read-only), computes a per-meter baseline, runs the detectors, classifies each finding with generic rules and returns daily metrics plus prioritized anomalies with evidence. No LLM yet: explanations and recommendations come in the next phase. Internal service: no published ports; the backend reaches it at `AI_ENGINE_URL`.

```
engine-ai/
├── app/
│   ├── main.py          # composition root: settings, JSON logging, DB pool, app
│   ├── api.py           # GET /health, POST /analyze
│   ├── loader.py        # read-only SQL -> DataFrames (only module touching the DB)
│   └── analysis/        # pure core: DataFrames in, result out, no I/O
│       ├── pipeline.py  # runs the steps, yields progress events and the result
│       ├── baseline.py  # reference window, hour-of-day profile, daily metrics
│       ├── detectors.py # persistent change, transient, outliers, pattern, electrical, data quality
│       ├── events.py    # event matching
│       ├── classify.py  # type, severity, confidence, priority
│       ├── thresholds.py
│       └── models.py
└── tests/               # unit, acceptance on the seed CSVs, API, integration
```

Local development (DB in Docker, engine on the host, Python 3.12):

```bash
docker compose up -d db
cd engine-ai
make install           # creates .venv with runtime + dev dependencies
make run               # loads ../.env, serves on localhost:8000
make test              # unit + acceptance tests, no Docker needed
make lint              # ruff check + ruff format --check
make test-integration  # loader against the dockerized DB
```

Endpoints:

| Method | Path | Body | Description |
|---|---|---|---|
| `GET` | `/health` | — | `200 {"status":"ok"}` if the database responds, `503 {"error":"database unavailable"}` otherwise |
| `POST` | `/analyze` | optional `{"analysis_id": 7}` (log correlation only) | NDJSON stream: one `step` line per step, then one `result` (or `error`) line |

```bash
curl -N -X POST localhost:8000/analyze
```

```
{"type": "step", "step": "READINGS"}
{"type": "step", "step": "BASELINE"}
{"type": "step", "step": "DETECTION"}
{"type": "step", "step": "CORRELATION"}
{"type": "step", "step": "EVENTS"}
{"type": "result", "metrics": [...], "anomalies": [...]}
```

A load failure before the stream starts returns `503`; an exception during the analysis ends the stream with `{"type": "error", "message": "analysis failed"}` (details only in the logs). `metrics` has one row per meter; `anomalies` only the meters with a finding. Numbers are rounded to 2 decimals, timestamps are RFC3339 UTC. Example anomaly (`outlier_timestamps` shortened):

```json
{"meter_id": "M-109", "anomaly": true, "type": "REAL_ANOMALY", "severity": "HIGH", "confidence": 0.99, "priority": 1,
 "evidence": {"baseline_kwh": 1052.7, "current_kwh": 2207.6, "variation_pct": 109.71,
   "change_start": "2026-09-12T14:00:00Z", "baseline_reliable": true, "shift_pct": 109.3, "effect_size": 29.71,
   "changed_vars": {"power_factor": {"before": 0.94, "after": 0.74}, "current_a": {"before": 195.35, "after": 411.07},
                    "voltage_v": {"before": 219.84, "after": 216.99}},
   "transient": null, "failed_checks": [], "outlier_timestamps": ["2026-09-12T14:00:00Z", "..."],
   "profile_correlation": 0.98, "related_event_ids": [3],
   "signals": ["no_explaining_event", "power_factor_drop", "power_ratio_shift", "current_follows"]}}
```

### Detection approach

Detection is rules plus robust statistics, per meter, with no meter-specific logic; every threshold is a named constant in `app/analysis/thresholds.py`.

- **Baseline (two passes).** Hour-of-day profile `P[h]` = median kWh at each hour over a reference window; `ratio = kWh / P[hour]` removes the daily cycle. Pass 1 uses the whole series; if a persistent change is found, pass 2 uses the full days before it (at least 3), so the baseline is not contaminated by the change. `baseline_kwh` = median daily total of the reference window (a one-day outage does not pull it down), `current_kwh` = last 24 hours, `variation_pct` = their relative difference.
- **Persistent change.** Best single mean-shift split of the ratio (both segments ≥ 12 h): flagged when the median shift is ≥ 20 %, the effect size is ≥ 5 and the change lasts ≥ 24 h.
- **Transient.** Longest run of hours with the ratio ≥ 30 % away from 1, lasting 3–23 h and followed by ≥ 3 normal hours.
- **Outliers and hourly pattern** (evidence only): hours with a robust z ≥ 8 and the correlation of the last day's shape with the reference profile.
- **Electrical change** at the change split: power-factor drop ≥ 0.05 or a shift ≥ 15 % of `kWh / (V·I·PF/1000)`; current moving with consumption is recorded as a supporting signal.
- **Data quality**: voltage > 5 % from its median, voltage / power-factor jumps (robust z > 6), relative current jumps while consumption stays flat, `kWh` inconsistent with `V·I·PF` by > 50 %, power factor outside [0, 1], ≥ 6 identical consecutive values. Fires when ≥ 3 flagged hours fall inside a 24 h window.
- **Events** of the same meter: a change or data-quality onset matches an event within ±6 h; a transient matches a `SCHEDULED_OUTAGE` only if it lies fully inside `[event − 6 h, event + 24 h]`. `OPERATIONAL_CHANGE` explains a persistent change, `SCHEDULED_OUTAGE` a transient, `DATA_QUALITY` corroborates a data-quality finding; `UNKNOWN` explains nothing.

### Classification (first matching rule wins)

| # | Condition | `type` | `severity` |
|---|---|---|---|
| 1 | Data-quality checks fire, no persistent change | `DATA_QUALITY` | `HIGH` |
| 2 | Persistent change + matching `OPERATIONAL_CHANGE` event | `EXPLAINABLE_ANOMALY` | `MEDIUM` |
| 3 | Persistent change, no explaining event | `REAL_ANOMALY` | `HIGH` if shift ≥ 100 % or electrical change, else `MEDIUM` |
| 4 | Transient fully inside a `SCHEDULED_OUTAGE` window | `FALSE_POSITIVE` (`anomaly: false`) | `LOW` |
| 5 | Transient, no explaining event | `REAL_ANOMALY` | `MEDIUM` |
| 6 | Nothing | no anomaly | — |

**Confidence** = `min(0.99, 0.50 + 0.30 · strength + 0.05 · corroborations)`, minus `0.15` when the baseline is not reliable. `strength` ∈ [0, 1] is `|effect| / 20` for changes and transients and `flagged hours / 12` for data quality; every corroboration is listed in `evidence.signals`.

**Priority**: `REAL_ANOMALY` > `DATA_QUALITY` > `EXPLAINABLE_ANOMALY` > `FALSE_POSITIVE`, then severity, confidence, `|variation_pct|` and `meter_id`.

Result on the seed data (the other 8 meters come out normal):

| Priority | Meter | Type | Severity | Confidence |
|---|---|---|---|---|
| 1 | M-109 | `REAL_ANOMALY` | `HIGH` | 0.99 |
| 2 | M-112 | `DATA_QUALITY` | `HIGH` | 0.99 |
| 3 | M-104 | `EXPLAINABLE_ANOMALY` | `MEDIUM` | 0.83 |
| 4 | M-106 | `FALSE_POSITIVE` | `LOW` | 0.85 |

**Known limitation:** a change within the first 3 days has no clean pre-change reference. It is still detected, but flagged `baseline_reliable: false` and its confidence is lowered.

## Roadmap

1. ✅ **Base**: docker-compose, PostgreSQL, schema, CSV load and backend skeleton (`/health`).
2. ✅ **Backend**: meters and readings endpoints.
3. ✅ **engine-ai**: baseline, detection and classification.
4. **AI**: LLM explanation and recommendation; analysis endpoints.
5. **Frontend**: dashboard, meters, detail, anomalies and investigation.
6. **Quality**: tests, documentation and demo script.

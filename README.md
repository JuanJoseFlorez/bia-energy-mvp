# BIA Energy MVP — AI Energy Management Platform

MVP to manage 12 electric meters (14 days of hourly readings) and use AI to **detect, explain, prioritize and recommend actions** on consumption anomalies.

## Architecture

```
Frontend (React/TS) ──HTTP──▶ Backend (Go, REST API) ──HTTP──▶ engine-ai (Python)
                                     │                               │
                                     └─────────▶ PostgreSQL ◀────────┘
```

| Component    | Stack                                                        | Role                                                    |
| ------------ | ------------------------------------------------------------ | ------------------------------------------------------- |
| `frontend/`  | Vite, React 19, TypeScript, Tailwind CSS v4, TanStack Query, Recharts | Dashboard, meters, detail, AI anomalies, investigation |
| `backend/`   | Go 1.22, `net/http`, pgx                                     | REST API, persistence, orchestrates analysis runs       |
| `engine-ai/` | Python 3.12, FastAPI, pandas, LiteLLM                        | Baseline, detection, classification, explanations       |
| `db`         | PostgreSQL 15                                                | Seeded from `db-init/` (`readings.csv`, `events.csv`)   |

- **Detection is deterministic; the LLM only explains.** Statistics and rules decide type, severity, confidence and priority; the LLM writes the texts from the computed evidence and can never change the classification.
- **Works without an LLM.** With no `LLM_API_KEY`, template explanations are used; the demo never depends on a provider.
- **Provider-agnostic.** The LLM is `LLM_MODEL=provider/model` through LiteLLM (e.g. `gemini/gemini-3.8-flash`, `openai/gpt-4o-mini`).
- **`engine-ai` is internal** (no published ports); the frontend talks only to the backend.

## Getting started

```bash
cp .env.example .env      # set DB_USER, DB_PASSWORD, DB_NAME; optionally LLM_MODEL + LLM_API_KEY
docker compose up --build
```

Open http://localhost:3000 and sign in with `demo` / `demo` (`DEMO_USER` / `DEMO_PASSWORD`). The API is on http://localhost:8080.

`init.sql` runs only on an empty volume: after a schema change, `docker compose down -v` (data reloads from the CSVs).

## Demo flow

`Login → Dashboard → Run AI Analysis → Medidores → M-109 → Anomalías IA → Investigación → Acción`

1. **Run AI Analysis** (header, every screen): a banner shows the seven pipeline steps live (`Lecturas → Baseline → Detección → Correlación → Eventos → Explicación → Recomendación`) and ends with *"4 anomalías detectadas · 2 requieren atención prioritaria"*.
2. **Dashboard**: KPIs (meters, consumption, AI anomalies, high priority, AI confidence, last analysis) and *"Qué investigar primero"*, in priority order.
3. **Medidores**: filters (all / normal / alert / critical), search, sort; **detail** with consumption vs baseline (hourly or daily), change window, outliers, events, and voltage / current / power factor.
4. **Anomalías IA**: type, severity, confidence and recommended action per anomaly.
5. **Investigación**: what the AI found (LLM or template), baseline comparison, changed variables, evidence, related events — and the action buttons (`PENDING → INVESTIGATING → VALIDATED → RESOLVED`, or `DISMISSED`).

Result on the seed data (the other 8 meters come out normal):

| Priority | Meter | Type | Severity | Confidence | Action |
|---|---|---|---|---|---|
| 1 | M-109 | `REAL_ANOMALY` | `HIGH` | 0.99 | Investigar medidor e instalación |
| 2 | M-112 | `DATA_QUALITY` | `HIGH` | 0.99 | Validar medidor / lecturas |
| 3 | M-104 | `EXPLAINABLE_ANOMALY` | `MEDIUM` | 0.83 | Validar operación |
| 4 | M-106 | `FALSE_POSITIVE` | `LOW` | 0.85 | No escalar |

## How the AI works

**Detection** (per meter, no meter-specific logic; thresholds in `engine-ai/app/analysis/thresholds.py`):

- **Baseline**: hour-of-day median profile over a reference window; recomputed on the days before a detected change so the change does not contaminate it. `baseline_kwh` = median daily total, `current_kwh` = last 24 h.
- **Persistent change**: best mean-shift split of consumption / profile (≥ 20 % shift, ≥ 24 h). **Transient**: 3–23 h deviation ≥ 30 %.
- **Electrical change**: power-factor drop or a shift in `kWh / (V·I·PF)` at the change.
- **Data quality**: voltage out of range, voltage / current / power-factor jumps with flat consumption, `kWh` inconsistent with `V·I·PF`, repeated values.
- **Events** of the same meter within ±6 h (outages: `[event − 6 h, event + 24 h]`); `UNKNOWN` events explain nothing.

**Classification** (first matching rule wins):

| Condition | Type | Severity |
|---|---|---|
| Data-quality checks fire, no persistent change | `DATA_QUALITY` | `HIGH` |
| Persistent change + `OPERATIONAL_CHANGE` event | `EXPLAINABLE_ANOMALY` | `MEDIUM` |
| Persistent change, no explaining event | `REAL_ANOMALY` | `HIGH` if ≥ 100 % or electrical change, else `MEDIUM` |
| Transient inside a `SCHEDULED_OUTAGE` window | `FALSE_POSITIVE` | `LOW` |
| Transient, no explaining event | `REAL_ANOMALY` | `MEDIUM` |

**Confidence** = `min(0.99, 0.50 + 0.30 · strength + 0.05 · corroborations)` (− 0.15 with an unreliable baseline). **Priority**: `REAL_ANOMALY` > `DATA_QUALITY` > `EXPLAINABLE_ANOMALY` > `FALSE_POSITIVE`, then severity and confidence.

**Explanations**: the LLM gets only the evidence as Spanish facts and returns `reason`, `explanation` and an action detail. Guardrails:

- The action category is fixed by type (above), so a false positive can never be escalated.
- A draft is accepted only if it passes schema, length and **grounding** checks (every number, date and time must come from the evidence); otherwise the template is used. `explanation_source` records `llm` or `template`.
- Bounded pool (`LLM_MAX_CONCURRENCY`, default 4), deadline per analysis (`LLM_TIMEOUT_SECONDS`, default 20) with limited retries.

## API

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/login` | Demo login (mock) |
| `GET` | `/dashboard/summary` | Platform KPIs |
| `GET` | `/meters` | `status`, `q`, `sort` (`consumption`\|`variation`\|`severity`), `order` |
| `GET` | `/meters/{meterId}` | Metrics, health, anomaly, events |
| `GET` | `/meters/{meterId}/readings` | Hourly readings, optional `from` / `to` |
| `POST` | `/ai/analyze` | Start a run: `202` + `Location`, `409` if one is active |
| `GET` | `/ai/analysis/{id}` · `/ai/analysis/latest` | Status, current step, summary, error |
| `GET` | `/anomalies` | Latest run by default; `type`, `severity`, `status` |
| `GET` | `/anomalies/{id}` | Explanation, evidence, related events, readings window, next statuses |
| `PATCH` | `/anomalies/{id}` | `{"status": …}` along the action workflow (`409` if not allowed) |

Runs execute in the background: the backend consumes engine-ai's NDJSON progress stream, stores the current step, and writes metrics, anomalies and summary in one transaction. Timeouts, engine failures and shutdowns end the run `FAILED` with a readable `error`; a partial unique index guarantees one active run.

Errors share one format: `{"error": {"code": "not_found", "message": "..."}, "request_id": "..."}`.

## Development

Each service has a `Makefile` (`make run`, `make test`, `make lint`):

```bash
docker compose up -d db
make -C backend test test-integration     # Go unit + integration (dockerized DB)
make -C engine-ai install test            # pytest: unit + acceptance on the seed CSVs
make -C frontend install test             # Vitest + Testing Library
```

`make run` in `backend/` (`:8080`), `engine-ai/` (`:8000`) and `frontend/` (`:3000`) runs each service on the host against the dockerized DB. `engine-ai` also has `make test-llm` to try the real LLM configured in `.env`.

## Known limitations

- **Login is a mock**: it checks one demo user and returns a token that no endpoint verifies; the API is not protected.
- **Runs execute inside the backend process**: a crashed backend leaves its run active until a stale sweep fails it (`AI_ENGINE_TIMEOUT` + 30 s). Next step would be a database-backed job queue.
- **A change in the first 3 days** has no clean reference: it is detected, but flagged `baseline_reliable: false` with lower confidence.

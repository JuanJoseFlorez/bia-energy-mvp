# BIA Energy MVP — AI Energy Management Platform

MVP to manage electric meters and use AI to **detect, explain, prioritize and recommend actions** on consumption anomalies.

> Status: **work in progress**. Database and orchestration ready; services pending.

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
├── backend/            # Go REST API (pending)
├── frontend/           # React + TypeScript app (pending)
├── engine-ai/          # Python analysis engine (pending)
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

2. Start the database (only service available for now):

   ```bash
   docker compose up db
   ```

> `init.sql` runs only on an empty volume. To recreate the database after a schema change: `docker compose down -v && docker compose up db` (wipes the volume; data reloads from the CSVs).

## Roadmap

1. ✅ **Base**: docker-compose, PostgreSQL, schema and CSV load.
2. **Backend**: meters and readings endpoints.
3. **engine-ai**: baseline, detection and classification.
4. **AI**: LLM explanation and recommendation; analysis endpoints.
5. **Frontend**: dashboard, meters, detail, anomalies and investigation.
6. **Quality**: tests, documentation and demo script.

.PHONY: install run test test-integration test-llm lint

ENV_FILE ?= ../.env
PYTHON ?= python3.12
BIN := .venv/bin

# Creates the local virtualenv with runtime and dev dependencies.
install:
	$(PYTHON) -m venv .venv
	$(BIN)/pip install -r requirements-dev.txt

# Runs the engine locally against the dockerized DB (docker compose up db).
# LITELLM_LOCAL_MODEL_COST_MAP: use the bundled model cost map instead of fetching it at startup.
run:
	@set -a; . $(ENV_FILE); set +a; \
	DB_HOST=localhost DB_PORT=$${DB_HOST_PORT:-5432} LITELLM_LOCAL_MODEL_COST_MAP=True \
	$(BIN)/uvicorn app.main:app --port 8000

test:
	$(BIN)/pytest

# Runs the loader against the dockerized DB (docker compose up -d db).
test-integration:
	@set -a; . $(ENV_FILE); set +a; \
	TEST_DATABASE_URL="postgresql://$${DB_USER}:$${DB_PASSWORD}@localhost:$${DB_HOST_PORT:-5432}/$${DB_NAME}" \
	$(BIN)/pytest tests/test_loader_integration.py -v

# Asks the real LLM provider (LLM_MODEL / LLM_API_KEY from ../.env) to explain the seed anomalies.
test-llm:
	@set -a; . $(ENV_FILE); set +a; \
	LLM_LIVE=1 $(BIN)/pytest tests/test_llm_live.py -v -s

lint:
	$(BIN)/ruff check .
	$(BIN)/ruff format --check .

.PHONY: install run test test-integration lint

ENV_FILE ?= ../.env
PYTHON ?= python3.12
BIN := .venv/bin

# Creates the local virtualenv with runtime and dev dependencies.
install:
	$(PYTHON) -m venv .venv
	$(BIN)/pip install -r requirements-dev.txt

# Runs the engine locally against the dockerized DB (docker compose up db).
run:
	@set -a; . $(ENV_FILE); set +a; \
	DB_HOST=localhost DB_PORT=$${DB_HOST_PORT:-5432} $(BIN)/uvicorn app.main:app --port 8000

test:
	$(BIN)/pytest

# Runs the loader against the dockerized DB (docker compose up -d db).
test-integration:
	@set -a; . $(ENV_FILE); set +a; \
	TEST_DATABASE_URL="postgresql://$${DB_USER}:$${DB_PASSWORD}@localhost:$${DB_HOST_PORT:-5432}/$${DB_NAME}" \
	$(BIN)/pytest tests/test_loader_integration.py -v

lint:
	$(BIN)/ruff check .
	$(BIN)/ruff format --check .

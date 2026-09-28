.PHONY: run test test-integration lint build

ENV_FILE ?= ../.env

# Runs the API locally against the dockerized DB (docker compose up db) and a local engine-ai (cd ../engine-ai && make run).
run:
	@set -a; . $(ENV_FILE); set +a; \
	DB_HOST=localhost DB_PORT=$${DB_HOST_PORT:-5432} AI_ENGINE_URL=$${AI_ENGINE_URL:-http://localhost:8000} go run ./cmd/api

test:
	go test ./...

# Runs integration tests against the dockerized DB (docker compose up -d db).
test-integration:
	@set -a; . $(ENV_FILE); set +a; \
	TEST_DATABASE_URL="postgres://$${DB_USER}:$${DB_PASSWORD}@localhost:$${DB_HOST_PORT:-5432}/$${DB_NAME}?sslmode=disable" \
	go test ./... -run Integration -count=1 -p 1

lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...

build:
	go build -o bin/api ./cmd/api

.PHONY: run test lint build

ENV_FILE ?= ../.env

# Runs the API locally against the dockerized DB (docker compose up db).
run:
	@set -a; . $(ENV_FILE); set +a; \
	DB_HOST=localhost DB_PORT=$${DB_HOST_PORT:-5432} go run ./cmd/api

test:
	go test ./...

lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...

build:
	go build -o bin/api ./cmd/api

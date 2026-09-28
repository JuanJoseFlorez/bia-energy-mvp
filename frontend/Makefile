.PHONY: install run test lint build

install:
	npm ci

# Vite dev server on http://localhost:3000 against the backend at VITE_API_URL (default http://localhost:8080).
run:
	npm run dev

test:
	npm test

lint:
	npm run lint && npm run typecheck

build:
	npm run build

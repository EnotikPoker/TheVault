.PHONY: install db-up db-down dev-backend dev-frontend check test test-integration build fmt

GO ?= go
DOCKER ?= docker

db-up:
	$(DOCKER) compose up -d --wait

db-down:
	$(DOCKER) compose down

install:
	cd frontend && npm ci

dev-backend: export DATABASE_URL ?= postgres://thevault:thevault_dev@127.0.0.1:5433/thevault?sslmode=disable
dev-backend:
	cd backend && $(GO) run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

check:
	cd backend && $(GO) vet ./...
	cd frontend && npm run typecheck

test:
	cd backend && $(GO) test ./...
	cd frontend && npm test

test-integration: export TEST_DATABASE_URL ?= postgres://thevault:thevault_dev@127.0.0.1:5433/thevault?sslmode=disable
test-integration:
	cd backend && $(GO) test -race -tags=integration ./...

build:
	mkdir -p backend/bin
	cd backend && $(GO) build -o bin/server ./cmd/server
	cd frontend && npm run build

fmt:
	cd backend && $(GO) fmt ./...

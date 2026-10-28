.PHONY: install dev-backend dev-frontend check test build fmt

GO ?= go

install:
	cd frontend && npm ci

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

build:
	mkdir -p backend/bin
	cd backend && $(GO) build -o bin/server ./cmd/server
	cd frontend && npm run build

fmt:
	cd backend && $(GO) fmt ./...

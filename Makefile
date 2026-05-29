# AI HR Recruitment — local dev Makefile.
# Override DB_URL on the command line if your host connection differs:
#   make migrate-up DB_URL=postgres://...

DB_URL ?= postgres://hruser:hrpass@localhost:5432/hr_db?sslmode=disable
MIGRATIONS_DIR := backend/migrations
MIGRATE := migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)"
POSTGRES_USER ?= hruser
POSTGRES_DB ?= hr_db
PSQL := docker compose exec -T postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

.PHONY: help up down logs ps migrate-up migrate-down migrate-create seed import build run-api run-worker test test-integration lint vet tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS=":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Start the full stack (detached)
	docker compose up -d --build

down: ## Stop the stack
	docker compose down

logs: ## Tail stack logs
	docker compose logs -f

ps: ## Show service status
	docker compose ps

migrate-up: ## Apply all migrations
	$(MIGRATE) up

migrate-down: ## Roll back the last migration
	$(MIGRATE) down 1

migrate-create: ## Create a new migration: make migrate-create name=add_foo
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

seed: ## Load representative reference data via SQL (quick demo)
	$(PSQL) < scripts/seed_stores.sql
	$(PSQL) < scripts/seed_positions.sql
	$(PSQL) < scripts/seed_vacancies.sql

import: ## Import reference data from CSVs: make import DIR=scripts
	cd backend && go run ./cmd/importref ../$(DIR)/stores.sample.csv ../$(DIR)/positions.sample.csv

build: ## Build both binaries
	cd backend && go build ./...

run-api: ## Run the api locally (needs deps up + .env exported)
	cd backend && go run ./cmd/api

run-worker: ## Run the worker locally (needs deps up + .env exported)
	cd backend && go run ./cmd/worker

test: ## Run unit tests
	cd backend && go test -race ./... -cover

test-integration: ## Run integration tests (needs: make up && make migrate-up && make seed)
	cd backend && go test -tags integration ./... -count=1

lint: ## Run golangci-lint
	cd backend && golangci-lint run

vet: ## Run go vet
	cd backend && go vet ./...

tidy: ## Tidy go modules
	cd backend && go mod tidy

security: ## Run Go security scans (gosec + govulncheck)
	cd backend && gosec ./... && govulncheck ./...

e2e: ## Boot the stack, seed, and run the cross-system e2e suite
	docker compose up -d --build
	@echo "waiting for api health…"; for i in $$(seq 1 60); do curl -sf http://localhost:8080/health >/dev/null 2>&1 && break; sleep 1; done
	$(MAKE) migrate-up && $(MAKE) seed
	cd backend && go test -tags e2e ./e2e/... -count=1 -v

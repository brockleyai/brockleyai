# Default model for E2E tests. Override with: make test-e2e MODEL=anthropic/claude-sonnet-4-20250514
MODEL ?= openai/gpt-4o

.PHONY: dev dev-down test test-coverage test-e2e test-e2e-verbose test-e2e-llm-verbose test-e2e-no-llm test-e2e-no-mcp test-e2e-no-external test-e2e-cli-only test-e2e-tf-only lint build docker-build helm-install helm-upgrade helm-uninstall help

dev: ## Start full local development stack (PostgreSQL + Redis + server + worker + UI)
	docker compose -f deploy/docker/docker-compose.yml up --build

dev-down: ## Stop local development environment
	docker compose -f deploy/docker/docker-compose.yml down

dev-server: ## Start server only (requires PostgreSQL + Redis running)
	BROCKLEY_ENV=development go run ./cmd/server

dev-worker: ## Start worker only (requires PostgreSQL + Redis running)
	go run ./cmd/worker

test: ## Run all tests (no external services required)
	go test ./... -race -count=1

test-coverage: ## Run tests with coverage report
	go test ./... -race -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out
	@echo ""
	@echo "HTML report: go tool cover -html=coverage.out -o coverage.html"

test-integration: ## Run integration tests (requires DATABASE_URL)
	DATABASE_URL="postgres://brockley:brockley@localhost:5432/brockley?sslmode=disable" \
	go test ./server/store/postgres/... -v -count=1

test-e2e: ## Run all E2E tests (requires Docker, optionally OPENROUTER_API_KEY)
	MODEL=$(MODEL) bash tests/e2e/run.sh

test-e2e-verbose: ## Run all E2E tests with verbose/debug output
	MODEL=$(MODEL) bash tests/e2e/run.sh --verbose

test-e2e-llm-verbose: ## Run only LLM E2E tests and print persisted LLM request/response traces
	MODEL=$(MODEL) bash tests/e2e/run.sh --llm-verbose

test-e2e-no-llm: ## Run E2E tests without LLM graphs (no API key needed)
	MODEL=$(MODEL) bash tests/e2e/run.sh --no-llm

test-e2e-no-mcp: ## Run E2E tests without MCP graphs (Graphs 1-3 only)
	MODEL=$(MODEL) bash tests/e2e/run.sh --no-mcp

test-e2e-no-external: ## Run E2E tests without LLM or MCP graphs (Graphs 1, 2 only)
	MODEL=$(MODEL) bash tests/e2e/run.sh --no-llm --no-mcp

test-e2e-cli-only: ## Run E2E tests via CLI path only
	MODEL=$(MODEL) bash tests/e2e/run.sh --cli-only

test-e2e-tf-only: ## Run E2E tests via Terraform path only
	MODEL=$(MODEL) bash tests/e2e/run.sh --tf-only

lint: ## Run linters
	golangci-lint run ./...

build: ## Build server, worker, coderunner, and CLI binaries
	go build -o bin/brockley-server ./cmd/server
	go build -o bin/brockley-worker ./cmd/worker
	go build -o bin/brockley-coderunner ./cmd/coderunner

dev-coderunner: ## Start coderunner only (requires Redis running)
	go run ./cmd/coderunner

docker-build: ## Build production Docker images
	docker build -t brockleyai/brockley:latest -f deploy/docker/Dockerfile --target production .
	docker build -t brockleyai/brockley-worker:latest -f deploy/docker/Dockerfile.worker --target production .
	docker build -t brockleyai/brockley-coderunner:latest -f deploy/docker/Dockerfile.coderunner --target production .

helm-install: ## Install Brockley to current Kubernetes context
	helm install brockley ./deploy/helm/brockley

helm-upgrade: ## Upgrade existing Helm installation
	helm upgrade brockley ./deploy/helm/brockley

helm-uninstall: ## Remove Brockley from Kubernetes
	helm uninstall brockley

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help

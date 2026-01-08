.PHONY: help build run test lint clean docker-up docker-down migrate sqlc

# Default target
help:
	@echo "Temper - Adaptive AI Pairing Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build the API binary"
	@echo "  make run            Run the API server locally"
	@echo "  make test           Run unit tests"
	@echo "  make test-cover     Run unit tests with coverage report"
	@echo "  make test-integration  Run integration tests (requires Docker)"
	@echo "  make test-all       Run all tests including integration"
	@echo "  make lint           Run linters"
	@echo "  make clean          Clean build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up      Start all services with Docker Compose"
	@echo "  make docker-down    Stop all services"
	@echo "  make docker-build   Build Docker images"
	@echo "  make docker-logs    View logs from all services"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up     Run database migrations up"
	@echo "  make migrate-down   Rollback last migration"
	@echo "  make migrate-new    Create a new migration (NAME=migration_name)"
	@echo "  make sqlc           Generate Go code from SQL queries"
	@echo ""
	@echo "Development:"
	@echo "  make dev            Start development environment"
	@echo "  make deps           Download Go dependencies"
	@echo "  make fmt            Format Go code"

# Build
build:
	go build -o bin/temper ./cmd/temper

# Run locally
run:
	go run ./cmd/temper serve

# Test
test:
	go test -race -v ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Integration tests (requires Docker for testcontainers)
test-integration:
	go test -tags=integration -race -v -timeout 600s ./...

test-integration-cover:
	go test -tags=integration -race -coverprofile=coverage-integration.out -timeout 600s ./...
	go tool cover -html=coverage-integration.out -o coverage-integration.html

# Run all tests including integration
test-all:
	go test -tags=integration -race -v -timeout 600s ./...

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	gofmt -w .
	goimports -w .

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Dependencies
deps:
	go mod download
	go mod tidy

# Docker commands
docker-up:
	docker compose up -d postgres rabbitmq
	@echo "Waiting for services to be healthy..."
	@sleep 5
	docker compose up -d api

docker-down:
	docker compose down

docker-build:
	docker compose build

docker-logs:
	docker compose logs -f

docker-runner:
	docker compose --profile runner up -d runner

docker-ollama:
	docker compose --profile local-llm up -d ollama

# Database migrations
migrate-up:
	docker compose --profile migrate run --rm migrate

migrate-down:
	docker compose run --rm -e DATABASE_URL=postgres://temper:temper@postgres:5432/temper?sslmode=disable api ./temper migrate down

migrate-new:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-new NAME=migration_name"; exit 1; fi
	@VERSION=$$(ls -1 sql/migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	VERSION=$$(printf "%03d" $$((VERSION + 1))); \
	touch sql/migrations/$${VERSION}_$(NAME).up.sql; \
	touch sql/migrations/$${VERSION}_$(NAME).down.sql; \
	echo "Created sql/migrations/$${VERSION}_$(NAME).up.sql"; \
	echo "Created sql/migrations/$${VERSION}_$(NAME).down.sql"

# Generate sqlc code
sqlc:
	sqlc generate

# Development environment
dev: docker-up
	@echo "Development environment started!"
	@echo "  API: http://localhost:8080"
	@echo "  RabbitMQ UI: http://localhost:15672 (temper/temper)"
	@echo ""
	@echo "Run 'make migrate-up' to apply database migrations"

# Install development tools
install-tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Build runner base image
build-runner-image:
	docker build -t temper-runner-sandbox:latest ./docker/runner-image

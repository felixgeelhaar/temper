.PHONY: help build test test-cover test-integration test-all lint fmt clean deps install-tools eval eval-build build-runner-image

# Default target
help:
	@echo "Temper - Adaptive AI Pairing Tool"
	@echo ""
	@echo "Build:"
	@echo "  make build            Build temper + temperd binaries to bin/"
	@echo ""
	@echo "Test:"
	@echo "  make test             Run unit tests"
	@echo "  make test-cover       Unit tests with coverage report"
	@echo "  make test-integration Integration tests (requires Docker)"
	@echo "  make test-all         All tests including integration"
	@echo "  make eval             Run pairing eval harness against configured LLM (BYOK)"
	@echo ""
	@echo "Develop:"
	@echo "  make lint             Run golangci-lint"
	@echo "  make fmt              Format Go code"
	@echo "  make deps             Download Go dependencies"
	@echo "  make install-tools    Install dev tools (golangci-lint, goimports)"
	@echo "  make clean            Clean build artifacts"
	@echo ""
	@echo "Sandbox image:"
	@echo "  make build-runner-image  Build the Docker sandbox image used by temperd's runner"

# Build the CLI and daemon binaries.
build:
	go build -o bin/temper ./cmd/temper
	go build -o bin/temperd ./cmd/temperd

# Tests
test:
	go test -race ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration:
	go test -tags=integration -race -timeout 600s ./...

test-integration-cover:
	go test -tags=integration -race -coverprofile=coverage-integration.out -timeout 600s ./...
	go tool cover -html=coverage-integration.out -o coverage-integration.html

test-all:
	go test -tags=integration -race -timeout 600s ./...

# Lint and format
lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

# Dependencies
deps:
	go mod download
	go mod tidy

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Pairing evaluation harness
eval-build:
	go build -o bin/eval-harness ./cmd/eval-harness

eval: eval-build
	./bin/eval-harness -dir eval/cases -threshold 0.9

# Sandbox image used by internal/runner DockerExecutor
build-runner-image:
	docker build -t temper-runner-sandbox:latest ./docker/runner-image

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html coverage-integration.out coverage-integration.html

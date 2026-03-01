# Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
# This is licensed software from AccelByte Inc, for limitations
# and restrictions contact your company contract manager.

SHELL := /bin/bash

.PHONY: proto build lint lint-fix test test-coverage test-all test-integration test-integration-setup test-integration-teardown test-integration-run help

proto:
	docker run --tty --rm --user $$(id -u):$$(id -g) \
		--volume $$(pwd):/build \
		--workdir /build \
		--entrypoint /bin/bash \
		rvolosatovs/protoc:4.1.0 \
			proto.sh

build: proto

# Linting targets
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix ./...

# Unit testing targets
test:
	@echo "Running unit tests..."
	@go test $$(go list ./... | grep -v /tests/integration) -v

test-coverage:
	@echo "Running unit tests with coverage..."
	@go test $$(go list ./... | grep -v /tests/integration) -coverprofile=coverage.out
	@go tool cover -func=coverage.out | grep total

# Integration testing targets
test-integration-setup:
	@echo "Starting test database..."
	@docker-compose -f docker-compose.test.yml up -d postgres-test
	@echo "Waiting for database to be ready..."
	@sleep 5

test-integration-teardown:
	@echo "Stopping test database..."
	@docker-compose -f docker-compose.test.yml down -v

test-integration-run:
	@echo "Running integration tests..."
	@go test ./tests/integration/... -v -p 1

test-integration: test-integration-setup test-integration-run test-integration-teardown
	@echo "✅ Integration tests complete!"

# Run all checks (lint + unit tests + integration tests)
test-all: lint test-coverage test-integration
	@echo "✅ All checks passed!"

# Help
help:
	@echo "extend-challenge-service - Available Targets:"
	@echo ""
	@echo "Linting:"
	@echo "  make lint              Run golangci-lint"
	@echo "  make lint-fix          Run golangci-lint with auto-fix"
	@echo ""
	@echo "Unit Testing:"
	@echo "  make test              Run unit tests (excludes integration)"
	@echo "  make test-coverage     Run unit tests with coverage report"
	@echo ""
	@echo "Integration Testing:"
	@echo "  make test-integration-setup     Start test database container"
	@echo "  make test-integration-run       Run integration tests"
	@echo "  make test-integration-teardown  Stop and remove test database"
	@echo "  make test-integration           All-in-one: setup, run, teardown"
	@echo ""
	@echo "All Checks:"
	@echo "  make test-all          Run lint + unit coverage + integration tests"
	@echo ""
	@echo "Build:"
	@echo "  make proto             Generate protobuf code"
	@echo "  make build             Build (runs proto first)"
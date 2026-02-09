# Makefile for LLM Control Plane
# Enterprise-grade build automation for Go-based cloud-native middleware

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Build configuration
APP_NAME := api-gateway
BIN_DIR := bin
CMD_DIR := backend/cmd
INTERNAL_DIR := backend/internal
COVERAGE_DIR := coverage

# Go configuration
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod
GOFMT := gofmt
GOVET := $(GO) vet

# Build flags
BUILD_FLAGS := -ldflags="-s -w"
BUILD_FLAGS += -ldflags="-X main.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
BUILD_FLAGS += -ldflags="-X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"
BUILD_FLAGS += -ldflags="-X main.GitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# Docker configuration
DOCKER_COMPOSE := docker compose
COMPOSE_FILE := docker-compose.yml

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

# =============================================================================
# Help target
# =============================================================================

.PHONY: help
help: ## Show this help message
	@echo -e "$(COLOR_BOLD)LLM Control Plane - Available Commands$(COLOR_RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""

# =============================================================================
# Development workflow
# =============================================================================

.PHONY: setup
setup: ## Initial project setup (install deps, start infra)
	@echo -e "$(COLOR_GREEN)Setting up development environment...$(COLOR_RESET)"
	@$(MAKE) deps
	@$(MAKE) infra-up
	@echo -e "$(COLOR_GREEN)✓ Setup complete!$(COLOR_RESET)"
	@echo -e "$(COLOR_YELLOW)Run 'make dev' to start the development server$(COLOR_RESET)"

.PHONY: deps
deps: ## Download and verify Go dependencies
	@echo -e "$(COLOR_BLUE)Downloading dependencies...$(COLOR_RESET)"
	@$(GOMOD) download
	@$(GOMOD) verify
	@$(GOMOD) tidy

.PHONY: dev
dev: ## Run development server with hot reload
	@echo -e "$(COLOR_GREEN)Starting development server...$(COLOR_RESET)"
	@$(GO) run $(CMD_DIR)/api-gateway/main.go

# =============================================================================
# Build targets
# =============================================================================

.PHONY: build
build: clean ## Build the application binary
	@echo -e "$(COLOR_BLUE)Building $(APP_NAME)...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@$(GOBUILD) $(BUILD_FLAGS) -o $(BIN_DIR)/$(APP_NAME) $(CMD_DIR)/api-gateway/main.go
	@echo -e "$(COLOR_GREEN)✓ Build complete: $(BIN_DIR)/$(APP_NAME)$(COLOR_RESET)"

.PHONY: build-lambda
build-lambda: clean ## Build Lambda deployment package (Linux AMD64)
	@echo -e "$(COLOR_BLUE)Building Lambda function...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(BUILD_FLAGS) \
		-o $(BIN_DIR)/bootstrap $(CMD_DIR)/api-gateway/main.go
	@cd $(BIN_DIR) && zip -q lambda.zip bootstrap
	@echo -e "$(COLOR_GREEN)✓ Lambda package: $(BIN_DIR)/lambda.zip$(COLOR_RESET)"

.PHONY: build-all
build-all: build build-lambda ## Build all targets (local + Lambda)

# =============================================================================
# Testing
# =============================================================================

.PHONY: test
test: ## Run all tests
	@echo -e "$(COLOR_BLUE)Running tests...$(COLOR_RESET)"
	@$(GOTEST) -race -timeout 30s ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo -e "$(COLOR_BLUE)Running tests (verbose)...$(COLOR_RESET)"
	@$(GOTEST) -v -race -timeout 30s ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo -e "$(COLOR_BLUE)Running tests with coverage...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@$(GOTEST) -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo -e "$(COLOR_GREEN)✓ Coverage report: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)"

.PHONY: test-integration
test-integration: ## Run integration tests (requires infra)
	@echo -e "$(COLOR_BLUE)Running integration tests...$(COLOR_RESET)"
	@$(GOTEST) -v -tags=integration -timeout 60s ./...

# =============================================================================
# Code quality
# =============================================================================

.PHONY: fmt
fmt: ## Format Go code
	@echo -e "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	@$(GOFMT) -s -w $(CMD_DIR) $(INTERNAL_DIR)
	@echo -e "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet
	@echo -e "$(COLOR_BLUE)Running go vet...$(COLOR_RESET)"
	@$(GOVET) ./...
	@echo -e "$(COLOR_GREEN)✓ Vet passed$(COLOR_RESET)"

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo -e "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m; \
		echo -e "$(COLOR_GREEN)✓ Lint passed$(COLOR_RESET)"; \
	else \
		echo -e "$(COLOR_YELLOW)⚠ golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(COLOR_RESET)"; \
	fi

.PHONY: check
check: fmt vet lint test ## Run all quality checks

# =============================================================================
# Infrastructure management
# =============================================================================

.PHONY: infra-up
infra-up: ## Start local infrastructure (Postgres + Redis)
	@echo -e "$(COLOR_BLUE)Starting infrastructure...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d
	@echo -e "$(COLOR_GREEN)✓ Infrastructure running$(COLOR_RESET)"
	@echo -e "  Postgres: localhost:5432"
	@echo -e "  Redis:    localhost:6379"

.PHONY: infra-down
infra-down: ## Stop local infrastructure
	@echo -e "$(COLOR_BLUE)Stopping infrastructure...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down
	@echo -e "$(COLOR_GREEN)✓ Infrastructure stopped$(COLOR_RESET)"

.PHONY: infra-reset
infra-reset: ## Reset infrastructure (destroy volumes)
	@echo -e "$(COLOR_YELLOW)Resetting infrastructure (all data will be lost)...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down -v
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d
	@echo -e "$(COLOR_GREEN)✓ Infrastructure reset$(COLOR_RESET)"

.PHONY: infra-logs
infra-logs: ## Show infrastructure logs
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) logs -f

.PHONY: infra-status
infra-status: ## Show infrastructure status
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) ps

# =============================================================================
# Database management
# =============================================================================

.PHONY: db-migrate
db-migrate: ## Run database migrations (TODO: implement)
	@echo -e "$(COLOR_YELLOW)TODO: Implement database migrations$(COLOR_RESET)"

.PHONY: db-seed
db-seed: ## Seed database with test data (TODO: implement)
	@echo -e "$(COLOR_YELLOW)TODO: Implement database seeding$(COLOR_RESET)"

.PHONY: db-reset
db-reset: ## Reset database (drop + recreate + migrate)
	@echo -e "$(COLOR_YELLOW)Resetting database...$(COLOR_RESET)"
	@docker exec -it llm-cp-postgres psql -U dev -d postgres -c "DROP DATABASE IF EXISTS llm_control_plane_dev;"
	@docker exec -it llm-cp-postgres psql -U dev -d postgres -c "CREATE DATABASE llm_control_plane_dev;"
	@echo -e "$(COLOR_GREEN)✓ Database reset$(COLOR_RESET)"
	@$(MAKE) db-migrate

.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	@docker exec -it llm-cp-postgres psql -U dev -d llm_control_plane_dev

# =============================================================================
# Cleanup
# =============================================================================

.PHONY: clean
clean: ## Remove build artifacts
	@echo -e "$(COLOR_BLUE)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BIN_DIR) $(COVERAGE_DIR)
	@$(GOCLEAN)
	@echo -e "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

.PHONY: clean-all
clean-all: clean infra-down ## Remove all artifacts and stop infrastructure
	@echo -e "$(COLOR_GREEN)✓ Full cleanup complete$(COLOR_RESET)"

# =============================================================================
# Utilities
# =============================================================================

.PHONY: version
version: ## Show version information
	@echo -e "$(COLOR_BOLD)Version Information$(COLOR_RESET)"
	@echo "  Version:    $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')"
	@echo "  Commit:     $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "  Branch:     $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown')"
	@echo "  Go Version: $(shell $(GO) version)"
	@echo "  Build Time: $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')"

.PHONY: deps-update
deps-update: ## Update all dependencies to latest versions
	@echo -e "$(COLOR_BLUE)Updating dependencies...$(COLOR_RESET)"
	@$(GO) get -u ./...
	@$(GOMOD) tidy
	@echo -e "$(COLOR_GREEN)✓ Dependencies updated$(COLOR_RESET)"

.PHONY: deps-graph
deps-graph: ## Generate dependency graph (requires graphviz)
	@echo -e "$(COLOR_BLUE)Generating dependency graph...$(COLOR_RESET)"
	@$(GO) mod graph | modgraphviz | dot -Tpng -o deps-graph.png
	@echo -e "$(COLOR_GREEN)✓ Dependency graph: deps-graph.png$(COLOR_RESET)"

# =============================================================================
# CI/CD helpers
# =============================================================================

.PHONY: ci-test
ci-test: ## Run CI test suite
	@$(MAKE) deps
	@$(MAKE) check
	@$(MAKE) test-coverage

.PHONY: ci-build
ci-build: ## Build for CI/CD pipeline
	@$(MAKE) build-lambda


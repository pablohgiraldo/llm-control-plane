# Makefile for LLM Control Plane
# Enterprise-grade build automation for Go-based cloud-native middleware

# =============================================================================
# Include Modular Makefiles
# =============================================================================

include make/config.mk
include make/help.mk
include make/database.mk
include make/dev.mk
include make/ports.mk

# NOTE: Development workflow targets (setup, deps, dev) are now in make/dev.mk

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
check: fmt vet lint ## Run all quality checks

# =============================================================================
# Infrastructure management
# =============================================================================

.PHONY: infra-up
infra-up: ## Start local infrastructure (Postgres)
	@echo -e "$(COLOR_BLUE)Starting infrastructure...$(COLOR_RESET)"
	@docker rm -f $(DB_CONTAINER) 2>/dev/null || true
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d
	@echo -e "$(COLOR_GREEN)✓ Infrastructure running$(COLOR_RESET)"
	@echo -e "  Postgres: localhost:5432"

.PHONY: infra-down
infra-down: ## Stop local infrastructure
	@echo -e "$(COLOR_BLUE)Stopping infrastructure...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down
	@echo -e "$(COLOR_GREEN)✓ Infrastructure stopped$(COLOR_RESET)"

.PHONY: infra-reset
infra-reset: ## Reset infrastructure (destroy volumes)
	@echo -e "$(COLOR_YELLOW)Resetting infrastructure (all data will be lost)...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down -v
	@docker rm -f $(DB_CONTAINER) 2>/dev/null || true
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d
	@echo -e "$(COLOR_GREEN)✓ Infrastructure reset$(COLOR_RESET)"

.PHONY: infra-logs
infra-logs: ## Show infrastructure logs
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) logs -f

.PHONY: infra-status
infra-status: ## Show infrastructure status
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) ps

# NOTE: Database management targets are now in make/database.mk

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

.PHONY: ci-build
ci-build: ## Build for CI/CD pipeline
	@$(MAKE) build-lambda


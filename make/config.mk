# =============================================================================
# Project Configuration
# =============================================================================

# Project metadata
PROJECT_NAME := llm-control-plane
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build configuration
APP_NAME := api-gateway
BIN_DIR := bin
CMD_DIR := backend/cmd
INTERNAL_DIR := backend/internal
COVERAGE_DIR := coverage

# Frontend configuration
FRONTEND_DIR := frontend
FRONTEND_PORT := 5173

# Backend configuration
BACKEND_PORT := 8443  # Changed to 8443 for HTTPS
BACKEND_USE_HTTPS := true

# TLS Certificate configuration
CERTS_DIR := certs
CERT_FILE := $(CERTS_DIR)/cert.pem
KEY_FILE := $(CERTS_DIR)/key.pem
CERT_SCRIPT := scripts/generate-certs.ps1

# =============================================================================
# Tool Definitions
# =============================================================================

# Shell
SHELL := /bin/bash

# Go tools
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod
GOFMT := gofmt
GOVET := $(GO) vet

# Docker tools
DOCKER := docker
DOCKER_COMPOSE := docker compose
COMPOSE_FILE := backend/docker-compose.yml

# Database tools
MIGRATE := migrate
MIGRATE_DIR := backend/migrations
PSQL := docker exec -it llm-cp-postgres psql

# =============================================================================
# Database Configuration
# =============================================================================

DB_HOST := localhost
DB_PORT := 5432
DB_NAME := audit
DB_USER := dev
DB_PASSWORD := audit_password
DB_CONTAINER := llm-cp-postgres
DB_URL := postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# =============================================================================
# Build Flags
# =============================================================================

BUILD_FLAGS := -ldflags="-s -w"
BUILD_FLAGS += -ldflags="-X main.Version=$(VERSION)"
BUILD_FLAGS += -ldflags="-X main.BuildTime=$(BUILD_TIME)"
BUILD_FLAGS += -ldflags="-X main.GitCommit=$(GIT_COMMIT)"

# =============================================================================
# Colors for Terminal Output
# =============================================================================

COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_DIM := \033[2m
COLOR_RED := \033[31m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_MAGENTA := \033[35m
COLOR_CYAN := \033[36m

# =============================================================================
# Helper Functions
# =============================================================================

# Check if a command exists
define check_command
	@command -v $(1) >/dev/null 2>&1 || { \
		echo -e "$(COLOR_RED)Error: $(1) is not installed$(COLOR_RESET)"; \
		exit 1; \
	}
endef

# Print a colored message
define print_success
	echo -e "$(COLOR_GREEN)✓ $(1)$(COLOR_RESET)"
endef

define print_info
	echo -e "$(COLOR_BLUE)→ $(1)$(COLOR_RESET)"
endef

define print_warning
	echo -e "$(COLOR_YELLOW)⚠ $(1)$(COLOR_RESET)"
endef

define print_error
	echo -e "$(COLOR_RED)✗ $(1)$(COLOR_RESET)"
endef

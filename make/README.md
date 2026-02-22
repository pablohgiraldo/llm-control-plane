# Modular Makefile Structure

This directory contains modular Makefile components for the LLM Control Plane project.

## Structure Overview

```
make/
├── config.mk      # Configuration variables, colors, and tool definitions
├── help.mk        # Help system with categorized command display
├── database.mk    # PostgreSQL database management targets
├── dev.mk         # Development workflow targets
├── testing.mk     # Testing targets (unit, integration, coverage)
└── README.md      # This file
```

## Module Descriptions

### `config.mk` - Project Configuration
Contains all configuration variables and helper functions:
- **Project Metadata**: Version, git commit, build time
- **Build Configuration**: Directories, app names, ports
- **Tool Definitions**: Go, Docker, Migrate commands
- **Database Configuration**: Connection strings, container names
- **Build Flags**: Linker flags with version information
- **Colors**: Terminal output colors for better UX
- **Helper Functions**: `print_success`, `print_info`, `print_warning`, `print_error`

### `help.mk` - Help System
Enhanced help target that:
- Displays a beautiful categorized command list
- Groups commands by type (Development, Database, Testing, etc.)
- Uses color coding for better readability
- Includes `help-short` for quick reference

### `database.mk` - Database Management
Complete PostgreSQL management:
- **`db-up`**: Start PostgreSQL container
- **`db-down`**: Stop PostgreSQL container
- **`wait-for-db`**: Wait for database to be ready (60s timeout)
- **`migrate-up`**: Run database migrations using golang-migrate
- **`migrate-down`**: Rollback last migration
- **`migrate-reset`**: Reset all migrations
- **`migrate-create`**: Create new migration file
- **`reset-db`**: Complete database reset (drop, create, migrate, seed)
- **`db-seed`**: Seed database with test data
- **`db-shell`**: Open PostgreSQL interactive shell
- **`db-dump`**: Dump database to file
- **`db-restore`**: Restore database from file
- **`db-logs`**: Show PostgreSQL container logs
- **`db-status`**: Show database connection status

### `dev.mk` - Development Workflow
Development server management:
- **`dev`**: Run both backend and frontend servers concurrently
- **`backend-dev`**: Run backend server (Go) on port 8080
- **`frontend-dev`**: Run frontend server (Vite) on port 5173
- **`dev-backend-only`**: Start infrastructure + backend only
- **`dev-frontend-only`**: Start frontend only
- **`setup`**: Initial project setup (deps + infra + guidance)
- **`deps`**: Download and verify Go dependencies
- **`deps-frontend`**: Install frontend dependencies
- **`deps-all`**: Install all dependencies
- **`dev-watch`**: Run backend with Air hot reload
- **`dev-clean`**: Clean development artifacts

### `testing.mk` - Testing
Comprehensive testing targets:
- **`backend-test`**: Run Go unit tests with race detector
- **`backend-test-verbose`**: Verbose test output
- **`backend-test-coverage`**: Generate coverage report with HTML
- **`backend-integration-test`**: Run integration tests (requires database)
- **`frontend-test`**: Run frontend tests
- **`frontend-test-watch`**: Watch mode for frontend tests
- **`frontend-test-coverage`**: Frontend coverage report
- **`test-all`**: Run complete test suite
- **`test-quick`**: Quick unit tests only
- **`test-watch`**: Watch mode using gotestsum
- **`test-benchmark`**: Run Go benchmarks
- **`test-race`**: Race detection tests
- **`test-e2e`**: End-to-end tests (placeholder)
- **`test-report`**: Generate comprehensive test report

## Usage

### Quick Start

```bash
# Initial setup
make setup

# Start development servers
make dev

# Run tests
make test-all

# Database operations
make db-up
make migrate-up
make db-seed
```

### Common Workflows

#### New Developer Setup
```bash
make setup              # Install deps and start infrastructure
make migrate-up         # Run database migrations
make db-seed            # Seed with test data
make dev                # Start development servers
```

#### Database Migration Workflow
```bash
# Create new migration
make migrate-create NAME=add_users_table

# Apply migrations
make migrate-up

# Rollback last migration
make migrate-down

# Reset database completely
make reset-db
```

#### Testing Workflow
```bash
# Quick unit tests
make test-quick

# Full test suite with integration
make test-all

# Coverage report
make backend-test-coverage

# Watch mode during development
make test-watch
```

#### Development Workflow
```bash
# Run both frontend and backend
make dev

# Backend only (with infrastructure)
make dev-backend-only

# Frontend only
make dev-frontend-only
```

## Main Makefile

The main `Makefile` includes all modules at the top:

```makefile
include make/config.mk
include make/help.mk
include make/database.mk
include make/dev.mk
include make/testing.mk
```

It still contains unique targets not in modules:
- **Build targets**: `build`, `build-lambda`, `build-all`
- **Code quality**: `fmt`, `vet`, `lint`, `check`
- **Infrastructure**: `infra-up`, `infra-down`, `infra-reset`, `infra-logs`, `infra-status`
- **Cleanup**: `clean`, `clean-all`
- **Utilities**: `version`, `deps-update`, `deps-graph`
- **CI/CD**: `ci-test`, `ci-build`

## Helper Functions

All modules can use helper functions defined in `config.mk`:

```makefile
$(call print_success,"Message")  # Green checkmark
$(call print_info,"Message")     # Blue arrow
$(call print_warning,"Message")  # Yellow warning symbol
$(call print_error,"Message")    # Red X symbol
```

## Configuration Variables

Key variables from `config.mk`:

```makefile
# Project
PROJECT_NAME := llm-control-plane
APP_NAME := api-gateway

# Directories
CMD_DIR := backend/cmd
FRONTEND_DIR := frontend
MIGRATE_DIR := backend/migrations

# Ports
BACKEND_PORT := 8080
FRONTEND_PORT := 5173

# Database
DB_HOST := localhost
DB_PORT := 5432
DB_NAME := audit
DB_USER := dev
DB_PASSWORD := audit_password
DB_CONTAINER := llm-cp-postgres
```

## Dependencies

### Required Tools
- **Go**: Go compiler and tools
- **Docker**: For PostgreSQL and infrastructure
- **Node.js/npm**: For frontend development

### Optional Tools
- **golang-migrate**: For database migrations
  - Install: `scoop install migrate` (Windows) or `brew install golang-migrate` (macOS)
- **Air**: For backend hot reload
  - Install: `go install github.com/cosmtrek/air@latest`
- **gotestsum**: For test watch mode
  - Install: `go install gotest.tools/gotestsum@latest`
- **golangci-lint**: For linting
  - Install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

## Extending the Structure

To add new modules:

1. Create new `.mk` file in `make/` directory
2. Add include statement in main `Makefile`
3. Use configuration from `config.mk`
4. Follow naming conventions (e.g., `module-action`)
5. Add `## Help text` to all `.PHONY` targets

Example:

```makefile
# make/docker.mk
.PHONY: docker-build
docker-build: ## Build Docker image
	$(call print_info,"Building Docker image...")
	docker build -t $(PROJECT_NAME):$(VERSION) .
	$(call print_success,"Docker image built")
```

Then add to main Makefile:
```makefile
include make/docker.mk
```

## Color Output

The help system uses color-coded categories:
- 🟢 **Green**: Development commands
- 🔵 **Blue**: Database commands
- 🟣 **Magenta**: Testing commands
- 🟡 **Yellow**: Build & Quality commands
- 🔵 **Cyan**: Infrastructure commands
- ⚪ **Dim**: Utilities

## Best Practices

1. **Always use helper functions** for consistent output
2. **Check for tool availability** before running commands
3. **Provide helpful error messages** with installation instructions
4. **Use meaningful target names** with clear help text
5. **Group related targets** in the same module
6. **Avoid duplicating targets** across modules

## Troubleshooting

### "Command not found: migrate"
Install golang-migrate:
```bash
# Windows
scoop install migrate

# macOS
brew install golang-migrate

# From source
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### "Database connection refused"
Ensure PostgreSQL is running:
```bash
make db-up
make wait-for-db
```

### "Permission denied" errors
On Windows with WSL, ensure Docker Desktop is running and integrated with WSL.

---

**Last Updated**: February 2026
**Maintainers**: LLM Control Plane Team

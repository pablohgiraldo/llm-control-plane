# =============================================================================
# Database Management
# =============================================================================

.PHONY: db-up
db-up: ## Start PostgreSQL container
	$(call print_info,"Starting PostgreSQL...")
	@docker rm -f $(DB_CONTAINER) 2>/dev/null || true
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d postgres
	@$(MAKE) wait-for-postgres
	@$(MAKE) ensure-audit-db
	@$(MAKE) wait-for-db
	$(call print_success,"PostgreSQL is ready at $(DB_HOST):$(DB_PORT)")
	@echo -e "  $(COLOR_DIM)Database: $(DB_NAME)$(COLOR_RESET)"
	@echo -e "  $(COLOR_DIM)User:     $(DB_USER)$(COLOR_RESET)"

.PHONY: db-down
db-down: ## Stop PostgreSQL container
	$(call print_info,"Stopping PostgreSQL...")
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) stop postgres
	$(call print_success,"PostgreSQL stopped")

.PHONY: db-reset
db-reset: ## Remove container and volume, then start fresh (use when changing credentials)
	$(call print_warning,"Resetting database - removing container and volume...")
	@$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down -v 2>/dev/null || true
	@$(DOCKER) rm -f $(DB_CONTAINER) 2>/dev/null || true
	@$(DOCKER) volume rm backend_postgres-data 2>/dev/null || true
	@$(DOCKER) volume rm llm-control-plane_postgres-data 2>/dev/null || true
	@$(MAKE) db-up
	$(call print_success,"Database reset complete. Run make migrate-up if needed.")

.PHONY: wait-for-postgres
wait-for-postgres: ## Wait for PostgreSQL server to accept connections
	$(call print_info,"Waiting for PostgreSQL server...")
	@timeout=60; \
	while ! $(DOCKER) exec $(DB_CONTAINER) pg_isready -U $(DB_USER) -d postgres >/dev/null 2>&1; do \
		timeout=$$((timeout - 1)); \
		if [ $$timeout -le 0 ]; then \
			$(call print_error,"PostgreSQL failed to start within 60 seconds"); \
			exit 1; \
		fi; \
		sleep 1; \
	done

.PHONY: ensure-audit-db
ensure-audit-db: ## Create audit database if it does not exist
	@$(DOCKER) exec $(DB_CONTAINER) psql -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);" 2>/dev/null || true

.PHONY: wait-for-db
wait-for-db: ## Wait for audit database to be ready
	$(call print_info,"Waiting for database to be ready...")
	@timeout=60; \
	while ! $(DOCKER) exec $(DB_CONTAINER) pg_isready -U $(DB_USER) -d $(DB_NAME) >/dev/null 2>&1; do \
		timeout=$$((timeout - 1)); \
		if [ $$timeout -le 0 ]; then \
			$(call print_error,"Database failed to start within 60 seconds"); \
			exit 1; \
		fi; \
		sleep 1; \
	done
	$(call print_success,"Database is ready")

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	$(call print_info,"Running migrations up...")
	@if [ ! -d "$(MIGRATE_DIR)" ]; then \
		echo -e "$(COLOR_YELLOW)Migration directory not found, creating: $(MIGRATE_DIR)$(COLOR_RESET)"; \
		mkdir -p $(MIGRATE_DIR); \
	fi
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MIGRATE) -path $(MIGRATE_DIR) -database "$(DB_URL)" up; \
		$(call print_success,"Migrations applied successfully"); \
	else \
		$(call print_warning,"golang-migrate not installed"); \
		echo -e "  Install: $(COLOR_CYAN)brew install golang-migrate$(COLOR_RESET) (macOS)"; \
		echo -e "  Or:      $(COLOR_CYAN)go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest$(COLOR_RESET)"; \
		echo -e "  Windows: $(COLOR_CYAN)scoop install migrate$(COLOR_RESET)"; \
	fi

.PHONY: migrate-up-quiet
migrate-up-quiet: ## Run migrations (output suppressed, for make dev)
	@if [ ! -d "$(MIGRATE_DIR)" ]; then mkdir -p $(MIGRATE_DIR); fi
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MIGRATE) -path $(MIGRATE_DIR) -database "$(DB_URL)" up >/dev/null 2>&1; \
	fi

.PHONY: migrate-down
migrate-down: ## Rollback last database migration
	$(call print_warning,"Rolling back last migration...")
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MIGRATE) -path $(MIGRATE_DIR) -database "$(DB_URL)" down 1; \
		$(call print_success,"Migration rolled back"); \
	else \
		$(call print_error,"golang-migrate not installed (see migrate-up for installation)"); \
		exit 1; \
	fi

.PHONY: migrate-reset
migrate-reset: ## Reset all migrations (down then up)
	$(call print_warning,"Resetting all migrations...")
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MIGRATE) -path $(MIGRATE_DIR) -database "$(DB_URL)" down -all; \
		$(MIGRATE) -path $(MIGRATE_DIR) -database "$(DB_URL)" up; \
		$(call print_success,"Migrations reset complete"); \
	else \
		$(call print_error,"golang-migrate not installed"); \
		exit 1; \
	fi

.PHONY: migrate-create
migrate-create: ## Create new migration (usage: make migrate-create NAME=create_users_table)
	@if [ -z "$(NAME)" ]; then \
		$(call print_error,"NAME is required. Usage: make migrate-create NAME=create_users_table"); \
		exit 1; \
	fi
	$(call print_info,"Creating migration: $(NAME)")
	@mkdir -p $(MIGRATE_DIR)
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MIGRATE) create -ext sql -dir $(MIGRATE_DIR) -seq $(NAME); \
		$(call print_success,"Migration files created in $(MIGRATE_DIR)"); \
	else \
		$(call print_error,"golang-migrate not installed"); \
		exit 1; \
	fi

.PHONY: reset-db
reset-db: ## Reset database completely (drop, recreate, migrate, seed)
	$(call print_warning,"Resetting database (all data will be lost)...")
	@$(DOCKER) exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME);" 2>/dev/null || true
	@$(DOCKER) exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);"
	$(call print_success,"Database recreated")
	@if command -v $(MIGRATE) >/dev/null 2>&1; then \
		$(MAKE) migrate-up; \
	fi
	@$(MAKE) db-seed

.PHONY: db-seed
db-seed: ## Seed database with test data
	$(call print_info,"Seeding database with test data...")
	@if [ -f "$(MIGRATE_DIR)/seed.sql" ]; then \
		$(DOCKER) exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) < $(MIGRATE_DIR)/seed.sql; \
		$(call print_success,"Database seeded successfully"); \
	else \
		$(call print_warning,"No seed.sql file found in $(MIGRATE_DIR)"); \
		echo -e "  Create $(COLOR_CYAN)$(MIGRATE_DIR)/seed.sql$(COLOR_RESET) with your test data"; \
	fi

.PHONY: db-shell
db-shell: ## Open PostgreSQL interactive shell
	$(call print_info,"Opening database shell...")
	@$(DOCKER) exec -it $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME)

.PHONY: db-dump
db-dump: ## Dump database to file (usage: make db-dump FILE=backup.sql)
	@FILE=$${FILE:-backup_$(shell date +%Y%m%d_%H%M%S).sql}; \
	$(call print_info,"Dumping database to $$FILE..."); \
	$(DOCKER) exec $(DB_CONTAINER) pg_dump -U $(DB_USER) $(DB_NAME) > $$FILE; \
	$(call print_success,"Database dumped to $$FILE")

.PHONY: db-restore
db-restore: ## Restore database from file (usage: make db-restore FILE=backup.sql)
	@if [ -z "$(FILE)" ]; then \
		$(call print_error,"FILE is required. Usage: make db-restore FILE=backup.sql"); \
		exit 1; \
	fi
	$(call print_warning,"Restoring database from $(FILE)...")
	@$(DOCKER) exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) < $(FILE)
	$(call print_success,"Database restored from $(FILE)")

.PHONY: db-logs
db-logs: ## Show PostgreSQL logs
	@$(DOCKER) logs -f $(DB_CONTAINER)

.PHONY: db-status
db-status: ## Show database status and connection info
	@echo -e "$(COLOR_BOLD)Database Status:$(COLOR_RESET)"
	@if $(DOCKER) ps | grep -q $(DB_CONTAINER); then \
		echo -e "  Status:   $(COLOR_GREEN)Running$(COLOR_RESET)"; \
		echo -e "  Host:     $(DB_HOST):$(DB_PORT)"; \
		echo -e "  Database: $(DB_NAME)"; \
		echo -e "  User:     $(DB_USER)"; \
		echo -e "  URL:      $(COLOR_DIM)$(DB_URL)$(COLOR_RESET)"; \
	else \
		echo -e "  Status:   $(COLOR_RED)Stopped$(COLOR_RESET)"; \
		echo -e "  Run '$(COLOR_CYAN)make db-up$(COLOR_RESET)' to start"; \
	fi

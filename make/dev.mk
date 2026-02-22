# =============================================================================
# Development Workflow
# =============================================================================

.PHONY: certs
certs: ## Generate HTTPS certificates for local development (uses mkcert)
	@if [ -f $(CERTS_DIR)/cert.pem ] && [ -f $(CERTS_DIR)/key.pem ]; then \
		echo Certificates already exist in $(CERTS_DIR)/; \
	else \
		echo Generating HTTPS certificates with mkcert...; \
		mkdir -p $(CERTS_DIR); \
		if command -v mkcert >/dev/null 2>&1; then \
			mkcert -cert-file $(CERTS_DIR)/cert.pem -key-file $(CERTS_DIR)/key.pem localhost 127.0.0.1 ::1; \
			echo Certificates generated in $(CERTS_DIR)/; \
		else \
			echo "Error: mkcert not found. Install it: brew install mkcert (macOS) or chocolatey install mkcert (Windows)"; \
			exit 1; \
		fi; \
	fi

.PHONY: check-env
check-env: ## Verify backend/.env exists and has Cognito configuration
	@if [ ! -f backend/.env ]; then \
		echo ERROR: backend/.env not found!; \
		echo; \
		echo Please copy backend/.env.example backend/.env; \
		echo Then update with your AWS Cognito credentials.; \
		echo See docs/setup/COGNITO_DEPLOYMENT.md for details.; \
		exit 1; \
	fi; \
	echo Environment configuration OK

.PHONY: dev
dev: install db-up migrate-up-quiet certs ensure-ports-free ## Run minimal setup and start dev servers
	$(call print_info,"Starting development environment...")
	@echo -e "$(COLOR_CYAN)Backend (HTTPS): https://localhost:$(BACKEND_PORT)$(COLOR_RESET)"
	@echo -e "$(COLOR_CYAN)Frontend:        https://localhost:$(FRONTEND_PORT)$(COLOR_RESET)"
	@echo -e "$(COLOR_CYAN)Auth Login:      https://localhost:$(BACKEND_PORT)/auth/login$(COLOR_RESET)"
	@echo ""
	@echo -e "$(COLOR_YELLOW)Note: Accept self-signed certificate warnings in your browser$(COLOR_RESET)"
	@echo -e "$(COLOR_DIM)Press Ctrl+C to stop both servers$(COLOR_RESET)"
	@echo ""
	@trap 'kill 0' INT; \
	$(MAKE) backend-dev & \
	$(MAKE) frontend-dev & \
	wait

.PHONY: backend-dev
backend-dev: ## Run backend development server with HTTPS
	$(call print_info,"Starting backend server with HTTPS on :$(BACKEND_PORT)...")
	@echo -e "$(COLOR_DIM)Backend API: https://localhost:$(BACKEND_PORT)$(COLOR_RESET)"
	@cd backend && TLS_CERT_FILE="$(CURDIR)/$(CERT_FILE)" TLS_KEY_FILE="$(CURDIR)/$(KEY_FILE)" $(GO) run ./cmd/api-gateway

.PHONY: frontend-dev
frontend-dev: ## Run frontend development server (Vite)
	$(call print_info,"Starting frontend server on :$(FRONTEND_PORT)...")
	@echo -e "$(COLOR_DIM)Frontend UI: http://localhost:$(FRONTEND_PORT)$(COLOR_RESET)"
	@cd $(FRONTEND_DIR) && if [ -f pnpm-lock.yaml ]; then pnpm dev; else npm run dev; fi

.PHONY: dev-backend-only
dev-backend-only: infra-up backend-dev ## Start infra + backend only

.PHONY: dev-frontend-only
dev-frontend-only: frontend-dev ## Start frontend only

.PHONY: setup
setup: ## Initial project setup (install deps, start infra)
	@echo -e "$(COLOR_BOLD)$(COLOR_CYAN)Setting up LLM Control Plane development environment...$(COLOR_RESET)"
	@echo ""
	$(call print_info,"Installing backend dependencies...")
	@$(MAKE) deps
	$(call print_info,"Installing frontend dependencies...")
	@cd $(FRONTEND_DIR) && if [ -f pnpm-lock.yaml ]; then pnpm install; else npm install; fi
	$(call print_info,"Starting infrastructure...")
	@$(MAKE) infra-up
	@$(MAKE) wait-for-db
	$(call print_info,"Running database migrations...")
	@$(MAKE) migrate-up
	@echo ""
	$(call print_success,"Setup complete!")
	@echo ""
	@echo -e "$(COLOR_BOLD)Next steps:$(COLOR_RESET)"
	@echo -e "  1. Deploy Cognito:    Push to main or run GitHub Actions workflow"
	@echo -e "  2. Get credentials:   From GitHub Actions outputs or AWS Console"
	@echo -e "  3. Configure .env:    $(COLOR_CYAN)copy backend\.env.example backend\.env$(COLOR_RESET)"
	@echo -e "                        Then update with Cognito credentials"
	@echo -e "  4. Start dev servers: $(COLOR_CYAN)make dev$(COLOR_RESET)"
	@echo -e "     $(COLOR_DIM)HTTPS certs: run $(COLOR_CYAN)make certs$(COLOR_DIM) if needed, or set TLS_ENABLED=false to run without HTTPS$(COLOR_RESET)"
	@echo ""
	@echo -e "$(COLOR_YELLOW)See docs/setup/COGNITO_DEPLOYMENT.md for detailed instructions$(COLOR_RESET)"
	@echo ""

.PHONY: deps
deps: ## Download and verify Go dependencies
	$(call print_info,"Downloading Go dependencies...")
	@cd backend && $(GOMOD) download && $(GOMOD) verify && $(GOMOD) tidy
	$(call print_success,"Dependencies installed")

.PHONY: deps-frontend
deps-frontend: ## Install frontend dependencies
	$(call print_info,"Installing frontend dependencies...")
	@cd $(FRONTEND_DIR) && if [ -f pnpm-lock.yaml ]; then pnpm install; else npm install; fi
	$(call print_success,"Frontend dependencies installed")

.PHONY: deps-all
deps-all: deps deps-frontend ## Install all dependencies (backend + frontend)

.PHONY: install
install: ## Install all dependencies (frontend, backend, for make dev)
	$(call print_info,"Installing backend dependencies...")
	@$(MAKE) deps
	$(call print_info,"Installing frontend dependencies...")
	@cd $(FRONTEND_DIR) && if [ -f pnpm-lock.yaml ]; then pnpm install; else npm install; fi
	$(call print_success,"All dependencies installed")

.PHONY: dev-watch
dev-watch: ## Run backend (alias for backend-dev)
	$(MAKE) backend-dev

.PHONY: dev-clean
dev-clean: ## Clean development artifacts and restart
	$(call print_info,"Cleaning development environment...")
	@$(MAKE) clean
	@cd $(FRONTEND_DIR) && rm -rf node_modules/.vite dist
	$(call print_success,"Development environment cleaned")

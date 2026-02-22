# =============================================================================
# Help System
# =============================================================================

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message with all available commands
	@echo -e "$(COLOR_BOLD)$(COLOR_CYAN)╔════════════════════════════════════════════════════════════════╗$(COLOR_RESET)"
	@echo -e "$(COLOR_BOLD)$(COLOR_CYAN)║$(COLOR_RESET)  $(COLOR_BOLD)LLM Control Plane - Development Commands$(COLOR_RESET)                 $(COLOR_BOLD)$(COLOR_CYAN)║$(COLOR_RESET)"
	@echo -e "$(COLOR_BOLD)$(COLOR_CYAN)╚════════════════════════════════════════════════════════════════╝$(COLOR_RESET)"
	@echo ""
	@echo -e "$(COLOR_BOLD)Usage:$(COLOR_RESET) make $(COLOR_CYAN)<target>$(COLOR_RESET)"
	@echo ""
	@echo -e "$(COLOR_BOLD)Development:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '^(dev|install|backend-dev|frontend-dev|setup|certs):' | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(COLOR_BOLD)Database:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '^(db-|migrate-|wait-for-db|reset-db|ensure-ports-free):' | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(COLOR_BOLD)Build & Quality:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '^(build|clean|fmt|vet|lint|check):' | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_YELLOW)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(COLOR_BOLD)Infrastructure:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '^(infra-):' | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(COLOR_BOLD)Utilities:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		grep -E '^(version|deps):' | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_DIM)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "$(COLOR_DIM)Run 'make <target>' to execute a command$(COLOR_RESET)"
	@echo ""

.PHONY: help-short
help-short: ## Show abbreviated help
	@echo -e "$(COLOR_BOLD)Quick Commands:$(COLOR_RESET)"
	@echo -e "  $(COLOR_GREEN)make dev$(COLOR_RESET)        - Start development servers"
	@echo -e "  $(COLOR_BLUE)make db-up$(COLOR_RESET)      - Start database"
	@echo -e "  $(COLOR_YELLOW)make check$(COLOR_RESET)      - Format, vet, lint"
	@echo -e "  $(COLOR_YELLOW)make build$(COLOR_RESET)      - Build application"
	@echo ""
	@echo -e "Run '$(COLOR_CYAN)make help$(COLOR_RESET)' for full command list"

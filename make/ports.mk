# Ports Module
# Port checking and management for backend (8443) and frontend (5173)

.PHONY: check-port-available check-ports-available ensure-ports-free kill-port

check-port-available:
	@if [ -z "$(PORT)" ]; then \
		echo "Error: PORT parameter required. Usage: make check-port-available PORT=8443"; \
		exit 1; \
	fi
	@if [ "$$(uname -s 2>/dev/null || echo Windows)" = "Windows_NT" ] || echo "$${OS:-unknown}" | grep -qi windows; then \
		RESULT=$$(netstat -ano 2>/dev/null | findstr ":$(PORT) " | findstr "LISTENING" 2>/dev/null || true); \
		if [ -n "$$RESULT" ]; then \
			echo "Port $(PORT) is in use. Run: make kill-port PORT=$(PORT)"; \
			exit 1; \
		fi; \
	else \
		if lsof -i:$(PORT) >/dev/null 2>&1; then \
			echo "Port $(PORT) is in use. Run: make kill-port PORT=$(PORT)"; \
			exit 1; \
		fi; \
	fi
	@echo "Port $(PORT) is available"

check-ports-available:
	@$(MAKE) check-port-available PORT=$(BACKEND_PORT)
	@$(MAKE) check-port-available PORT=$(FRONTEND_PORT)
	@echo "Ports $(BACKEND_PORT) and $(FRONTEND_PORT) are available"

ensure-ports-free:
	@echo "Ensuring ports $(BACKEND_PORT) and $(FRONTEND_PORT) are free..."
	@$(MAKE) kill-port PORT=$(BACKEND_PORT) 2>/dev/null || true
	@$(MAKE) kill-port PORT=$(FRONTEND_PORT) 2>/dev/null || true
	@echo "Ports $(BACKEND_PORT) and $(FRONTEND_PORT) are ready"

kill-port:
	@if [ -z "$(PORT)" ]; then \
		echo "Error: PORT required. Usage: make kill-port PORT=8443"; \
		exit 1; \
	fi
	@if [ "$$(uname -s 2>/dev/null || echo Windows)" = "Windows_NT" ] || echo "$${OS:-unknown}" | grep -qi windows; then \
		PIDS=$$(netstat -ano 2>/dev/null | findstr ":$(PORT) " 2>/dev/null | awk '{print $$NF}' | sort -u | grep -v '^0$$' || true); \
		if [ -n "$$PIDS" ]; then for pid in $$PIDS; do taskkill //F //PID $$pid 2>/dev/null || true; done; fi; \
	else \
		PIDS=$$(lsof -ti:$(PORT) 2>/dev/null || true); \
		if [ -n "$$PIDS" ]; then for pid in $$PIDS; do kill -9 $$pid 2>/dev/null || true; done; fi; \
	fi

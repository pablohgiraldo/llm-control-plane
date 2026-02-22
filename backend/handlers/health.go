package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/upb/llm-control-plane/backend/app"
	"go.uber.org/zap"
)

// HealthCheck returns a simple health check handler
func HealthCheck(deps *app.Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// ReadinessCheck performs a more thorough readiness check
func ReadinessCheck(deps *app.Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := map[string]interface{}{
			"status": "ready",
			"checks": map[string]string{},
		}

		// Check database
		if deps.DB == nil {
			response["status"] = "not_ready"
			response["checks"].(map[string]string)["database"] = "not_initialized"
		} else if err := deps.DB.PingContext(ctx); err != nil {
			response["status"] = "not_ready"
			response["checks"].(map[string]string)["database"] = "unhealthy"
			deps.Logger.Error("database health check failed", zap.Error(err))
		} else {
			response["checks"].(map[string]string)["database"] = "healthy"
		}

		// Check providers
		providerCount := deps.ProviderRegistry.Count()
		if providerCount == 0 {
			response["checks"].(map[string]string)["providers"] = "none_configured"
		} else {
			response["checks"].(map[string]string)["providers"] = "configured"
		}

		w.Header().Set("Content-Type", "application/json")
		if response["status"] == "ready" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(response)
	}
}

// StatusHandler returns application status information
func StatusHandler(deps *app.Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"version":     "0.1.0",
			"environment": deps.Config.Environment,
			"providers":   deps.ProviderRegistry.List(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}
}

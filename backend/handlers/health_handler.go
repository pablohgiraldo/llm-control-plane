package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthHandler handles health-related HTTP requests
type HealthHandler struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(db *sql.DB, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		db:     db,
		logger: logger,
	}
}

// HandleHealth handles GET /health
// Basic health check - always returns 200 if service is running
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	_ = utils.WriteOK(w, response)
}

// HandleReadiness handles GET /health/ready
// Readiness check - validates that all dependencies are available
func (h *HealthHandler) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	checks := make(map[string]string)
	allHealthy := true
	
	// Check database connectivity
	if err := h.checkDatabase(ctx); err != nil {
		h.logger.Warn("database health check failed", zap.Error(err))
		checks["database"] = "unhealthy"
		allHealthy = false
	} else {
		checks["database"] = "healthy"
	}
	
	// Determine overall status
	status := "healthy"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}
	
	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}
	
	if err := utils.WriteJSON(w, httpStatus, utils.SuccessResponse{Data: response}); err != nil {
		h.logger.Error("failed to write readiness response", zap.Error(err))
	}
}

// checkDatabase checks database connectivity
func (h *HealthHandler) checkDatabase(ctx context.Context) error {
	if h.db == nil {
		return nil // No database configured
	}
	
	// Ping database with timeout
	if err := h.db.PingContext(ctx); err != nil {
		return err
	}
	
	// Check if we can execute a simple query
	var result int
	if err := h.db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return err
	}
	
	return nil
}

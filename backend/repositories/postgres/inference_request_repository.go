package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// InferenceRequestRepository implements the repositories.InferenceRequestRepository interface
type InferenceRequestRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInferenceRequestRepository creates a new inference request repository
func NewInferenceRequestRepository(db *DB, logger *zap.Logger) repositories.InferenceRequestRepository {
	return &InferenceRequestRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new inference request
func (r *InferenceRequestRepository) Create(ctx context.Context, req *models.InferenceRequest) error {
	query := `
		INSERT INTO inference_requests (
			id, request_id, org_id, app_id, user_id, model, provider,
			prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
			status, error_message, created_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		req.ID,
		req.RequestID,
		req.OrgID,
		req.AppID,
		req.UserID,
		req.Model,
		req.Provider,
		req.PromptTokens,
		req.CompletionTokens,
		req.TotalTokens,
		req.Cost,
		req.LatencyMs,
		req.Status,
		req.ErrorMessage,
		req.CreatedAt,
		req.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create inference request: %w", err)
	}

	r.logger.Debug("inference request created", zap.String("id", req.ID.String()), zap.String("request_id", req.RequestID))
	return nil
}

// GetByID retrieves an inference request by ID
func (r *InferenceRequestRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	req := &models.InferenceRequest{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&req.ID,
		&req.RequestID,
		&req.OrgID,
		&req.AppID,
		&req.UserID,
		&req.Model,
		&req.Provider,
		&req.PromptTokens,
		&req.CompletionTokens,
		&req.TotalTokens,
		&req.Cost,
		&req.LatencyMs,
		&req.Status,
		&req.ErrorMessage,
		&req.CreatedAt,
		&req.CompletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("inference request not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get inference request: %w", err)
	}

	return req, nil
}

// GetByRequestID retrieves an inference request by external request ID
func (r *InferenceRequestRepository) GetByRequestID(ctx context.Context, requestID string) (*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE request_id = $1
	`

	executor := GetExecutor(ctx, r.db)
	req := &models.InferenceRequest{}

	err := executor.QueryRowContext(ctx, query, requestID).Scan(
		&req.ID,
		&req.RequestID,
		&req.OrgID,
		&req.AppID,
		&req.UserID,
		&req.Model,
		&req.Provider,
		&req.PromptTokens,
		&req.CompletionTokens,
		&req.TotalTokens,
		&req.Cost,
		&req.LatencyMs,
		&req.Status,
		&req.ErrorMessage,
		&req.CreatedAt,
		&req.CompletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("inference request not found: %s", requestID)
		}
		return nil, fmt.Errorf("failed to get inference request: %w", err)
	}

	return req, nil
}

// GetByOrgID retrieves inference requests for an organization with pagination
func (r *InferenceRequestRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryInferenceRequests(ctx, query, orgID, limit, offset)
}

// GetByAppID retrieves inference requests for an application with pagination
func (r *InferenceRequestRepository) GetByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE app_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryInferenceRequests(ctx, query, appID, limit, offset)
}

// GetByUserID retrieves inference requests for a user with pagination
func (r *InferenceRequestRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryInferenceRequests(ctx, query, userID, limit, offset)
}

// GetByStatus retrieves inference requests by status
func (r *InferenceRequestRepository) GetByStatus(ctx context.Context, orgID uuid.UUID, status models.InferenceStatus, limit, offset int) ([]*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE org_id = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	return r.queryInferenceRequests(ctx, query, orgID, status, limit, offset)
}

// GetByDateRange retrieves inference requests within a date range
func (r *InferenceRequestRepository) GetByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time, limit, offset int) ([]*models.InferenceRequest, error) {
	query := `
		SELECT id, request_id, org_id, app_id, user_id, model, provider,
		       prompt_tokens, completion_tokens, total_tokens, cost, latency_ms,
		       status, error_message, created_at, completed_at
		FROM inference_requests
		WHERE org_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`

	return r.queryInferenceRequests(ctx, query, orgID, start, end, limit, offset)
}

// Update updates an inference request
func (r *InferenceRequestRepository) Update(ctx context.Context, req *models.InferenceRequest) error {
	query := `
		UPDATE inference_requests
		SET status = $2,
		    model = $3,
		    provider = $4,
		    prompt_tokens = $5,
		    completion_tokens = $6,
		    total_tokens = $7,
		    cost = $8,
		    latency_ms = $9,
		    error_message = $10,
		    completed_at = $11
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query,
		req.ID,
		req.Status,
		req.Model,
		req.Provider,
		req.PromptTokens,
		req.CompletionTokens,
		req.TotalTokens,
		req.Cost,
		req.LatencyMs,
		req.ErrorMessage,
		req.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update inference request: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("inference request not found: %s", req.ID)
	}

	r.logger.Debug("inference request updated", zap.String("id", req.ID.String()))
	return nil
}

// GetMetrics retrieves aggregate metrics for an organization
func (r *InferenceRequestRepository) GetMetrics(ctx context.Context, orgID uuid.UUID, start, end time.Time) (*repositories.InferenceMetrics, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_requests,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_requests,
			COUNT(CASE WHEN status = 'rejected' THEN 1 END) as rejected_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost), 0) as total_cost,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM inference_requests
		WHERE org_id = $1 AND created_at >= $2 AND created_at <= $3
	`

	executor := GetExecutor(ctx, r.db)
	metrics := &repositories.InferenceMetrics{}

	err := executor.QueryRowContext(ctx, query, orgID, start, end).Scan(
		&metrics.TotalRequests,
		&metrics.CompletedRequests,
		&metrics.FailedRequests,
		&metrics.RejectedRequests,
		&metrics.TotalTokens,
		&metrics.TotalCost,
		&metrics.AvgLatencyMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	return metrics, nil
}

// WithTx returns a new repository instance bound to the transaction
func (r *InferenceRequestRepository) WithTx(tx repositories.Transaction) repositories.InferenceRequestRepository {
	return &InferenceRequestRepository{
		db:     r.db,
		logger: r.logger,
	}
}

// queryInferenceRequests is a helper method to query multiple inference requests
func (r *InferenceRequestRepository) queryInferenceRequests(ctx context.Context, query string, args ...interface{}) ([]*models.InferenceRequest, error) {
	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query inference requests: %w", err)
	}
	defer rows.Close()

	var requests []*models.InferenceRequest
	for rows.Next() {
		req := &models.InferenceRequest{}
		err := rows.Scan(
			&req.ID,
			&req.RequestID,
			&req.OrgID,
			&req.AppID,
			&req.UserID,
			&req.Model,
			&req.Provider,
			&req.PromptTokens,
			&req.CompletionTokens,
			&req.TotalTokens,
			&req.Cost,
			&req.LatencyMs,
			&req.Status,
			&req.ErrorMessage,
			&req.CreatedAt,
			&req.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inference request: %w", err)
		}
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inference request rows: %w", err)
	}

	return requests, nil
}

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

// AuditRepository implements the repositories.AuditRepository interface
type AuditRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *DB, logger *zap.Logger) repositories.AuditRepository {
	return &AuditRepository{
		db:     db,
		logger: logger,
	}
}

// Insert inserts a new audit log entry
// This method supports async insert patterns by being non-blocking
func (r *AuditRepository) Insert(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, org_id, app_id, user_id, action, resource_type, resource_id,
			details, ip_address, user_agent, request_id, timestamp,
			model, provider, tokens_used, cost, latency_ms, status_code, error_message
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		log.ID,
		log.OrgID,
		log.AppID,
		log.UserID,
		log.Action,
		log.ResourceType,
		log.ResourceID,
		log.Details,
		log.IPAddress,
		log.UserAgent,
		log.RequestID,
		log.Timestamp,
		log.Model,
		log.Provider,
		log.TokensUsed,
		log.Cost,
		log.LatencyMs,
		log.StatusCode,
		log.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	r.logger.Debug("audit log inserted", zap.String("id", log.ID.String()), zap.String("action", string(log.Action)))
	return nil
}

// GetByID retrieves an audit log by ID
func (r *AuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	log := &models.AuditLog{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.OrgID,
		&log.AppID,
		&log.UserID,
		&log.Action,
		&log.ResourceType,
		&log.ResourceID,
		&log.Details,
		&log.IPAddress,
		&log.UserAgent,
		&log.RequestID,
		&log.Timestamp,
		&log.Model,
		&log.Provider,
		&log.TokensUsed,
		&log.Cost,
		&log.LatencyMs,
		&log.StatusCode,
		&log.ErrorMessage,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audit log not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	return log, nil
}

// GetByOrgID retrieves audit logs for an organization with pagination
func (r *AuditRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE org_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryAuditLogs(ctx, query, orgID, limit, offset)
}

// GetByAppID retrieves audit logs for an application with pagination
func (r *AuditRepository) GetByAppID(ctx context.Context, appID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE app_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryAuditLogs(ctx, query, appID, limit, offset)
}

// GetByUserID retrieves audit logs for a user with pagination
func (r *AuditRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	return r.queryAuditLogs(ctx, query, userID, limit, offset)
}

// GetByDateRange retrieves audit logs within a date range
func (r *AuditRepository) GetByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE org_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp DESC
		LIMIT $4 OFFSET $5
	`

	return r.queryAuditLogs(ctx, query, orgID, start, end, limit, offset)
}

// GetByAction retrieves audit logs by action type
func (r *AuditRepository) GetByAction(ctx context.Context, orgID uuid.UUID, action models.AuditAction, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE org_id = $1 AND action = $2
		ORDER BY timestamp DESC
		LIMIT $3 OFFSET $4
	`

	return r.queryAuditLogs(ctx, query, orgID, action, limit, offset)
}

// GetByRequestID retrieves audit logs by request ID
func (r *AuditRepository) GetByRequestID(ctx context.Context, requestID string) ([]*models.AuditLog, error) {
	query := `
		SELECT id, org_id, app_id, user_id, action, resource_type, resource_id,
		       details, ip_address, user_agent, request_id, timestamp,
		       model, provider, tokens_used, cost, latency_ms, status_code, error_message
		FROM audit_logs
		WHERE request_id = $1
		ORDER BY timestamp DESC
	`

	return r.queryAuditLogs(ctx, query, requestID)
}

// WithTx returns a new repository instance bound to the transaction
func (r *AuditRepository) WithTx(tx repositories.Transaction) repositories.AuditRepository {
	return &AuditRepository{
		db:     r.db,
		logger: r.logger,
	}
}

// queryAuditLogs is a helper method to query multiple audit logs
func (r *AuditRepository) queryAuditLogs(ctx context.Context, query string, args ...interface{}) ([]*models.AuditLog, error) {
	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		err := rows.Scan(
			&log.ID,
			&log.OrgID,
			&log.AppID,
			&log.UserID,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&log.Details,
			&log.IPAddress,
			&log.UserAgent,
			&log.RequestID,
			&log.Timestamp,
			&log.Model,
			&log.Provider,
			&log.TokensUsed,
			&log.Cost,
			&log.LatencyMs,
			&log.StatusCode,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit log rows: %w", err)
	}

	return logs, nil
}

// AsyncInsert inserts an audit log entry asynchronously
// This is useful for high-throughput scenarios where blocking is not desired
func (r *AuditRepository) AsyncInsert(log *models.AuditLog) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := r.Insert(ctx, log); err != nil {
			r.logger.Error("failed to async insert audit log",
				zap.Error(err),
				zap.String("id", log.ID.String()),
				zap.String("action", string(log.Action)),
			)
		}
	}()
}

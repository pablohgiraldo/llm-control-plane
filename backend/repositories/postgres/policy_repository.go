package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// PolicyRepository implements the repositories.PolicyRepository interface
type PolicyRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewPolicyRepository creates a new policy repository
func NewPolicyRepository(db *DB, logger *zap.Logger) repositories.PolicyRepository {
	return &PolicyRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new policy
func (r *PolicyRepository) Create(ctx context.Context, policy *models.Policy) error {
	query := `
		INSERT INTO policies (id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		policy.ID,
		policy.OrgID,
		policy.AppID,
		policy.UserID,
		policy.PolicyType,
		policy.Config,
		policy.Priority,
		policy.Enabled,
		policy.CreatedAt,
		policy.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}

	r.logger.Debug("policy created", zap.String("id", policy.ID.String()))
	return nil
}

// GetByID retrieves a policy by ID
func (r *PolicyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error) {
	query := `
		SELECT id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at
		FROM policies
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	policy := &models.Policy{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&policy.ID,
		&policy.OrgID,
		&policy.AppID,
		&policy.UserID,
		&policy.PolicyType,
		&policy.Config,
		&policy.Priority,
		&policy.Enabled,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policy not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	return policy, nil
}

// GetByOrgID retrieves all policies for an organization
func (r *PolicyRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Policy, error) {
	query := `
		SELECT id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at
		FROM policies
		WHERE org_id = $1
		ORDER BY priority DESC, created_at DESC
	`

	return r.queryPolicies(ctx, query, orgID)
}

// GetByAppID retrieves all policies for an application
func (r *PolicyRepository) GetByAppID(ctx context.Context, appID uuid.UUID) ([]*models.Policy, error) {
	query := `
		SELECT id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at
		FROM policies
		WHERE app_id = $1
		ORDER BY priority DESC, created_at DESC
	`

	return r.queryPolicies(ctx, query, appID)
}

// GetByUserID retrieves all policies for a user
func (r *PolicyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Policy, error) {
	query := `
		SELECT id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at
		FROM policies
		WHERE user_id = $1
		ORDER BY priority DESC, created_at DESC
	`

	return r.queryPolicies(ctx, query, userID)
}

// GetApplicablePolicies retrieves all applicable policies for a request
// in priority order (org-level, app-level, user-level)
func (r *PolicyRepository) GetApplicablePolicies(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error) {
	// Build query based on whether userID is provided
	query := `
		SELECT id, org_id, app_id, user_id, policy_type, config, priority, enabled, created_at, updated_at
		FROM policies
		WHERE enabled = true
			AND org_id = $1
			AND (
				-- Org-level policies (no app or user specified)
				(app_id IS NULL AND user_id IS NULL)
				-- App-level policies
				OR (app_id = $2 AND user_id IS NULL)
	`

	args := []interface{}{orgID, appID}

	if userID != nil {
		query += `
				-- User-level policies
				OR (user_id = $3)
		`
		args = append(args, *userID)
	}

	query += `
			)
		ORDER BY 
			-- User-level policies first (highest priority)
			CASE WHEN user_id IS NOT NULL THEN 1
			     WHEN app_id IS NOT NULL THEN 2
			     ELSE 3 END,
			-- Then by priority within each level
			priority DESC,
			created_at DESC
	`

	return r.queryPolicies(ctx, query, args...)
}

// Update updates a policy
func (r *PolicyRepository) Update(ctx context.Context, policy *models.Policy) error {
	query := `
		UPDATE policies
		SET app_id = $2,
		    user_id = $3,
		    policy_type = $4,
		    config = $5,
		    priority = $6,
		    enabled = $7,
		    updated_at = $8
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query,
		policy.ID,
		policy.AppID,
		policy.UserID,
		policy.PolicyType,
		policy.Config,
		policy.Priority,
		policy.Enabled,
		policy.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("policy not found: %s", policy.ID)
	}

	r.logger.Debug("policy updated", zap.String("id", policy.ID.String()))
	return nil
}

// Delete deletes a policy
func (r *PolicyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM policies WHERE id = $1`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("policy not found: %s", id)
	}

	r.logger.Debug("policy deleted", zap.String("id", id.String()))
	return nil
}

// WithTx returns a new repository instance bound to the transaction
func (r *PolicyRepository) WithTx(tx repositories.Transaction) repositories.PolicyRepository {
	return &PolicyRepository{
		db:     r.db,
		logger: r.logger,
	}
}

// queryPolicies is a helper method to query multiple policies
func (r *PolicyRepository) queryPolicies(ctx context.Context, query string, args ...interface{}) ([]*models.Policy, error) {
	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}
	defer rows.Close()

	var policies []*models.Policy
	for rows.Next() {
		policy := &models.Policy{}
		err := rows.Scan(
			&policy.ID,
			&policy.OrgID,
			&policy.AppID,
			&policy.UserID,
			&policy.PolicyType,
			&policy.Config,
			&policy.Priority,
			&policy.Enabled,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}
		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating policy rows: %w", err)
	}

	return policies, nil
}

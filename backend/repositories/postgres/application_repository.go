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

// ApplicationRepository implements the repositories.ApplicationRepository interface
type ApplicationRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewApplicationRepository creates a new application repository
func NewApplicationRepository(db *DB, logger *zap.Logger) repositories.ApplicationRepository {
	return &ApplicationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new application
func (r *ApplicationRepository) Create(ctx context.Context, app *models.Application) error {
	query := `
		INSERT INTO applications (id, org_id, name, api_key_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		app.ID,
		app.OrgID,
		app.Name,
		app.APIKeyHash,
		app.CreatedAt,
		app.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	r.logger.Debug("application created", zap.String("id", app.ID.String()), zap.String("name", app.Name))
	return nil
}

// GetByID retrieves an application by ID
func (r *ApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Application, error) {
	query := `
		SELECT id, org_id, name, api_key_hash, created_at, updated_at
		FROM applications
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	app := &models.Application{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&app.ID,
		&app.OrgID,
		&app.Name,
		&app.APIKeyHash,
		&app.CreatedAt,
		&app.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("application not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return app, nil
}

// GetByAPIKeyHash retrieves an application by API key hash
func (r *ApplicationRepository) GetByAPIKeyHash(ctx context.Context, apiKeyHash string) (*models.Application, error) {
	query := `
		SELECT id, org_id, name, api_key_hash, created_at, updated_at
		FROM applications
		WHERE api_key_hash = $1
	`

	executor := GetExecutor(ctx, r.db)
	app := &models.Application{}

	err := executor.QueryRowContext(ctx, query, apiKeyHash).Scan(
		&app.ID,
		&app.OrgID,
		&app.Name,
		&app.APIKeyHash,
		&app.CreatedAt,
		&app.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("application not found for API key")
		}
		return nil, fmt.Errorf("failed to get application: %w", err)
	}

	return app, nil
}

// GetByOrgID retrieves all applications for an organization
func (r *ApplicationRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Application, error) {
	query := `
		SELECT id, org_id, name, api_key_hash, created_at, updated_at
		FROM applications
		WHERE org_id = $1
		ORDER BY created_at DESC
	`

	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query applications: %w", err)
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(
			&app.ID,
			&app.OrgID,
			&app.Name,
			&app.APIKeyHash,
			&app.CreatedAt,
			&app.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan application: %w", err)
		}
		apps = append(apps, app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating application rows: %w", err)
	}

	return apps, nil
}

// Update updates an application
func (r *ApplicationRepository) Update(ctx context.Context, app *models.Application) error {
	query := `
		UPDATE applications
		SET name = $2,
		    api_key_hash = $3,
		    updated_at = $4
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query,
		app.ID,
		app.Name,
		app.APIKeyHash,
		app.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("application not found: %s", app.ID)
	}

	r.logger.Debug("application updated", zap.String("id", app.ID.String()))
	return nil
}

// Delete deletes an application
func (r *ApplicationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM applications WHERE id = $1`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("application not found: %s", id)
	}

	r.logger.Debug("application deleted", zap.String("id", id.String()))
	return nil
}

// WithTx returns a new repository instance bound to the transaction
func (r *ApplicationRepository) WithTx(tx repositories.Transaction) repositories.ApplicationRepository {
	return &ApplicationRepository{
		db:     r.db,
		logger: r.logger,
	}
}

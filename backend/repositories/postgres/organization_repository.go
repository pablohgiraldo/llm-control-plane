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

// OrganizationRepository implements the repositories.OrganizationRepository interface
type OrganizationRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *DB, logger *zap.Logger) repositories.OrganizationRepository {
	return &OrganizationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new organization
func (r *OrganizationRepository) Create(ctx context.Context, org *models.Organization) error {
	query := `
		INSERT INTO organizations (id, name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		org.ID,
		org.Name,
		org.Slug,
		org.CreatedAt,
		org.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	r.logger.Debug("organization created", zap.String("id", org.ID.String()), zap.String("slug", org.Slug))
	return nil
}

// GetByID retrieves an organization by ID
func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	query := `
		SELECT id, name, slug, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	org := &models.Organization{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// GetBySlug retrieves an organization by slug
func (r *OrganizationRepository) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	query := `
		SELECT id, name, slug, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`

	executor := GetExecutor(ctx, r.db)
	org := &models.Organization{}

	err := executor.QueryRowContext(ctx, query, slug).Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization not found: %s", slug)
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// List retrieves all organizations with pagination
func (r *OrganizationRepository) List(ctx context.Context, limit, offset int) ([]*models.Organization, error) {
	query := `
		SELECT id, name, slug, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*models.Organization
	for rows.Next() {
		org := &models.Organization{}
		err := rows.Scan(
			&org.ID,
			&org.Name,
			&org.Slug,
			&org.CreatedAt,
			&org.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating organization rows: %w", err)
	}

	return orgs, nil
}

// Update updates an organization
func (r *OrganizationRepository) Update(ctx context.Context, org *models.Organization) error {
	query := `
		UPDATE organizations
		SET name = $2,
		    slug = $3,
		    updated_at = $4
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query,
		org.ID,
		org.Name,
		org.Slug,
		org.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("organization not found: %s", org.ID)
	}

	r.logger.Debug("organization updated", zap.String("id", org.ID.String()))
	return nil
}

// Delete deletes an organization
func (r *OrganizationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM organizations WHERE id = $1`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("organization not found: %s", id)
	}

	r.logger.Debug("organization deleted", zap.String("id", id.String()))
	return nil
}

// WithTx returns a new repository instance bound to the transaction
func (r *OrganizationRepository) WithTx(tx repositories.Transaction) repositories.OrganizationRepository {
	return &OrganizationRepository{
		db:     r.db,
		logger: r.logger,
	}
}

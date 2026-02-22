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

// UserRepository implements the repositories.UserRepository interface
type UserRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *DB, logger *zap.Logger) repositories.UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, cognito_sub, org_id, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	executor := GetExecutor(ctx, r.db)
	_, err := executor.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.CognitoSub,
		user.OrgID,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	r.logger.Debug("user created", zap.String("id", user.ID.String()), zap.String("email", user.Email))
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, cognito_sub, org_id, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	user := &models.User{}

	err := executor.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.CognitoSub,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByCognitoSub retrieves a user by Cognito subject
func (r *UserRepository) GetByCognitoSub(ctx context.Context, cognitoSub string) (*models.User, error) {
	query := `
		SELECT id, email, cognito_sub, org_id, role, created_at, updated_at
		FROM users
		WHERE cognito_sub = $1
	`

	executor := GetExecutor(ctx, r.db)
	user := &models.User{}

	err := executor.QueryRowContext(ctx, query, cognitoSub).Scan(
		&user.ID,
		&user.Email,
		&user.CognitoSub,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found for cognito_sub: %s", cognitoSub)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, cognito_sub, org_id, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	executor := GetExecutor(ctx, r.db)
	user := &models.User{}

	err := executor.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.CognitoSub,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found for email: %s", email)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByOrgID retrieves all users for an organization
func (r *UserRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.User, error) {
	query := `
		SELECT id, email, cognito_sub, org_id, role, created_at, updated_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at DESC
	`

	executor := GetExecutor(ctx, r.db)
	rows, err := executor.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.CognitoSub,
			&user.OrgID,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET email = $2,
		    role = $3,
		    updated_at = $4
		WHERE id = $1
	`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.Role,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", user.ID)
	}

	r.logger.Debug("user updated", zap.String("id", user.ID.String()))
	return nil
}

// Delete deletes a user
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	executor := GetExecutor(ctx, r.db)
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	r.logger.Debug("user deleted", zap.String("id", id.String()))
	return nil
}

// WithTx returns a new repository instance bound to the transaction
func (r *UserRepository) WithTx(tx repositories.Transaction) repositories.UserRepository {
	return &UserRepository{
		db:     r.db,
		logger: r.logger,
	}
}

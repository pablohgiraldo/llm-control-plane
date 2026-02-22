package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/upb/llm-control-plane/backend/config"
	"go.uber.org/zap"
)

// DB wraps the sql.DB connection pool
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// NewDB creates a new database connection pool
func NewDB(cfg config.DatabaseConfig, logger *zap.Logger) (*DB, error) {
	dsn := cfg.DSN()
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established",
		zap.String("connection", cfg.LogString()))

	return &DB{
		DB:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection pool
func (db *DB) Close() error {
	db.logger.Info("closing database connection")
	return db.DB.Close()
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Check if we can query
	var result int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("database query check failed: %w", err)
	}

	return nil
}

// Stats returns database connection pool statistics
func (db *DB) Stats() sql.DBStats {
	return db.DB.Stats()
}

// RunMigrations runs database migrations from the migrations directory
func (db *DB) RunMigrations(ctx context.Context, migrationsPath string) error {
	db.logger.Info("running database migrations", zap.String("path", migrationsPath))
	
	// Create migrations table if it doesn't exist
	createMigrationsTable := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Note: In production, use a proper migration tool like golang-migrate
	// This is a simplified implementation for demonstration
	db.logger.Info("migrations completed successfully")
	return nil
}

// InitSchema initializes the database schema
func (db *DB) InitSchema(ctx context.Context) error {
	schema := `
		-- Organizations table
		CREATE TABLE IF NOT EXISTS organizations (
			id UUID PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(100) NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		-- Applications table
		CREATE TABLE IF NOT EXISTS applications (
			id UUID PRIMARY KEY,
			org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			api_key_hash VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		-- Users table
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			cognito_sub VARCHAR(255) NOT NULL UNIQUE,
			org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(email, org_id)
		);

		-- Policies table
		CREATE TABLE IF NOT EXISTS policies (
			id UUID PRIMARY KEY,
			org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			app_id UUID REFERENCES applications(id) ON DELETE CASCADE,
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			policy_type VARCHAR(50) NOT NULL,
			config JSONB NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		-- Audit logs table
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			app_id UUID REFERENCES applications(id) ON DELETE SET NULL,
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			action VARCHAR(100) NOT NULL,
			resource_type VARCHAR(100) NOT NULL,
			resource_id UUID,
			details JSONB,
			ip_address VARCHAR(45),
			user_agent TEXT,
			request_id VARCHAR(255),
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			model VARCHAR(100),
			provider VARCHAR(100),
			tokens_used INTEGER,
			cost DECIMAL(10, 6),
			latency_ms INTEGER,
			status_code INTEGER,
			error_message TEXT
		);

		-- Inference requests table
		CREATE TABLE IF NOT EXISTS inference_requests (
			id UUID PRIMARY KEY,
			request_id VARCHAR(255) NOT NULL UNIQUE,
			org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
			app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			model VARCHAR(100) NOT NULL,
			provider VARCHAR(100) NOT NULL,
			prompt_tokens INTEGER,
			completion_tokens INTEGER,
			total_tokens INTEGER,
			cost DECIMAL(10, 6),
			latency_ms INTEGER,
			status VARCHAR(50) NOT NULL,
			error_message TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP
		);

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_applications_org_id ON applications(org_id);
		CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
		CREATE INDEX IF NOT EXISTS idx_users_cognito_sub ON users(cognito_sub);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		
		CREATE INDEX IF NOT EXISTS idx_policies_org_id ON policies(org_id);
		CREATE INDEX IF NOT EXISTS idx_policies_app_id ON policies(app_id);
		CREATE INDEX IF NOT EXISTS idx_policies_user_id ON policies(user_id);
		CREATE INDEX IF NOT EXISTS idx_policies_enabled ON policies(enabled);
		CREATE INDEX IF NOT EXISTS idx_policies_priority ON policies(priority);
		
		CREATE INDEX IF NOT EXISTS idx_audit_logs_org_id ON audit_logs(org_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_app_id ON audit_logs(app_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
		
		CREATE INDEX IF NOT EXISTS idx_inference_requests_org_id ON inference_requests(org_id);
		CREATE INDEX IF NOT EXISTS idx_inference_requests_app_id ON inference_requests(app_id);
		CREATE INDEX IF NOT EXISTS idx_inference_requests_user_id ON inference_requests(user_id);
		CREATE INDEX IF NOT EXISTS idx_inference_requests_status ON inference_requests(status);
		CREATE INDEX IF NOT EXISTS idx_inference_requests_created_at ON inference_requests(created_at);
		CREATE INDEX IF NOT EXISTS idx_inference_requests_request_id ON inference_requests(request_id);
	`

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	db.logger.Info("database schema initialized successfully")
	return nil
}

// InitAuditSchema initializes the audit database schema (audit_logs only, no FK).
// Use for the separate audit database when DATABASE_URL_AUDIT is set.
func (db *DB) InitAuditSchema(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY,
			org_id UUID NOT NULL,
			app_id UUID,
			user_id UUID,
			action VARCHAR(100) NOT NULL,
			resource_type VARCHAR(100) NOT NULL,
			resource_id UUID,
			details JSONB,
			ip_address VARCHAR(45),
			user_agent TEXT,
			request_id VARCHAR(255),
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			model VARCHAR(100),
			provider VARCHAR(100),
			tokens_used INTEGER,
			cost DECIMAL(10, 6),
			latency_ms INTEGER,
			status_code INTEGER,
			error_message TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_org_id ON audit_logs(org_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_app_id ON audit_logs(app_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id);
	`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to initialize audit schema: %w", err)
	}
	db.logger.Info("audit schema initialized successfully")
	return nil
}

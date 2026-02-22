package postgres

import (
	"context"

	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// RepositoryFactory creates and manages all repositories
type RepositoryFactory struct {
	db       *DB
	auditDB  *DB // Optional: separate DB for audit logs
	logger   *zap.Logger
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(cfg *config.Config, logger *zap.Logger) (*RepositoryFactory, error) {
	db, err := NewDB(cfg.Database, logger)
	if err != nil {
		return nil, err
	}

	f := &RepositoryFactory{db: db, logger: logger}

	if cfg.AuditDatabase != nil {
		auditDB, err := NewDB(*cfg.AuditDatabase, logger)
		if err != nil {
			db.Close()
			return nil, err
		}
		f.auditDB = auditDB
	}

	return f, nil
}

// InitAuditSchema initializes the audit database schema when using a separate audit DB.
func (f *RepositoryFactory) InitAuditSchema(ctx context.Context) error {
	if f.auditDB != nil {
		return f.auditDB.InitAuditSchema(ctx)
	}
	return nil
}

// NewRepositories creates all repository instances
func (f *RepositoryFactory) NewRepositories() *repositories.Repositories {
	auditDB := f.db
	if f.auditDB != nil {
		auditDB = f.auditDB
	}
	return &repositories.Repositories{
		Organizations:     NewOrganizationRepository(f.db, f.logger),
		Applications:      NewApplicationRepository(f.db, f.logger),
		Users:             NewUserRepository(f.db, f.logger),
		Policies:          NewPolicyRepository(f.db, f.logger),
		AuditLogs:         NewAuditRepository(auditDB, f.logger),
		InferenceRequests: NewInferenceRequestRepository(f.db, f.logger),
	}
}

// GetTransactionManager returns a transaction manager
func (f *RepositoryFactory) GetTransactionManager() repositories.TransactionManager {
	return NewTransactionManager(f.db, f.logger)
}

// GetDB returns the database connection
func (f *RepositoryFactory) GetDB() *DB {
	return f.db
}

// Close closes the database connection(s)
func (f *RepositoryFactory) Close() error {
	if f.auditDB != nil {
		_ = f.auditDB.Close()
	}
	return f.db.Close()
}

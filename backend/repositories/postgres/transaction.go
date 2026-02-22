package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// transactionContextKey is the context key for storing transactions
type transactionContextKey struct{}

// TransactionManager implements the TransactionManager interface
type TransactionManager struct {
	db     *DB
	logger *zap.Logger
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *DB, logger *zap.Logger) repositories.TransactionManager {
	return &TransactionManager{
		db:     db,
		logger: logger,
	}
}

// Begin starts a new transaction
func (tm *TransactionManager) Begin(ctx context.Context) (repositories.Transaction, error) {
	sqlTx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	tm.logger.Debug("transaction started")

	return &Transaction{
		tx:     sqlTx,
		ctx:    ctx,
		logger: tm.logger,
	}, nil
}

// InTransaction executes a function within a transaction
// Automatically commits if function succeeds, rolls back on error
func (tm *TransactionManager) InTransaction(ctx context.Context, fn func(ctx context.Context, tx repositories.Transaction) error) error {
	tx, err := tm.Begin(ctx)
	if err != nil {
		return err
	}

	// Create a new context with the transaction
	txCtx := context.WithValue(ctx, transactionContextKey{}, tx)

	// Execute the function
	if err := fn(txCtx, tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			tm.logger.Error("failed to rollback transaction",
				zap.Error(rbErr),
				zap.NamedError("original_error", err),
			)
		}
		return err
	}

	// Commit on success
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Transaction implements the Transaction interface
type Transaction struct {
	tx     *sql.Tx
	ctx    context.Context
	logger *zap.Logger
}

// Commit commits the transaction
func (t *Transaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	t.logger.Debug("transaction committed")
	return nil
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		// Ignore error if transaction is already closed
		if err == sql.ErrTxDone {
			return nil
		}
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	t.logger.Debug("transaction rolled back")
	return nil
}

// Context returns the transaction context
func (t *Transaction) Context() context.Context {
	return t.ctx
}

// GetTx returns the underlying sql.Tx for use by repositories
func (t *Transaction) GetTx() *sql.Tx {
	return t.tx
}

// GetTransactionFromContext retrieves a transaction from the context if available
func GetTransactionFromContext(ctx context.Context) (repositories.Transaction, bool) {
	tx, ok := ctx.Value(transactionContextKey{}).(repositories.Transaction)
	return tx, ok
}

// Executor is an interface that can execute queries (both *sql.DB and *sql.Tx)
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// GetExecutor returns the appropriate executor based on the context
// If a transaction is present in the context, it returns the transaction
// Otherwise, it returns the database connection
func GetExecutor(ctx context.Context, db *DB) Executor {
	if tx, ok := GetTransactionFromContext(ctx); ok {
		if pgTx, ok := tx.(*Transaction); ok {
			return pgTx.tx
		}
	}
	return db.DB
}

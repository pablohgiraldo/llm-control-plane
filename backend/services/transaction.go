package services

import (
	"context"
	"fmt"

	"github.com/upb/llm-control-plane/backend/repositories"
)

// WithTransaction executes a function within a database transaction.
// Automatically commits on success, rolls back on error.
// This follows the GrantPulse pattern for transaction management.
func WithTransaction(ctx context.Context, txMgr repositories.TransactionManager, fn func(ctx context.Context, tx repositories.Transaction) error) error {
	tx, err := txMgr.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Use defer to ensure rollback on panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-panic after rollback
		}
	}()

	// Execute the provided function
	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithTransactionResult executes a function within a database transaction and returns a result.
// Uses generics to support any return type.
// Automatically commits on success, rolls back on error.
func WithTransactionResult[T any](ctx context.Context, txMgr repositories.TransactionManager, fn func(ctx context.Context, tx repositories.Transaction) (T, error)) (T, error) {
	var result T

	tx, err := txMgr.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Use defer to ensure rollback on panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-panic after rollback
		}
	}()

	// Execute the provided function
	result, err = fn(ctx, tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return result, fmt.Errorf("transaction error: %v, rollback error: %w", err, rbErr)
		}
		return result, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

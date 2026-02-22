package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/upb/llm-control-plane/backend/repositories"
)

// MockTransactionManager is a mock implementation of TransactionManager
type MockTransactionManager struct {
	mock.Mock
}

func (m *MockTransactionManager) Begin(ctx context.Context) (repositories.Transaction, error) {
	args := m.Called(ctx)
	if tx := args.Get(0); tx != nil {
		return tx.(repositories.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTransactionManager) InTransaction(ctx context.Context, fn func(ctx context.Context, tx repositories.Transaction) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// MockTransaction is a mock implementation of Transaction
type MockTransaction struct {
	mock.Mock
	committed  bool
	rolledback bool
}

func (m *MockTransaction) Commit() error {
	args := m.Called()
	m.committed = true
	return args.Error(0)
}

func (m *MockTransaction) Rollback() error {
	args := m.Called()
	m.rolledback = true
	return args.Error(0)
}

func (m *MockTransaction) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func TestWithTransaction_Success(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Commit").Return(nil)

	err := WithTransaction(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) error {
		// Simulate successful operation
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, mockTx.committed)
	assert.False(t, mockTx.rolledback)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransaction_ErrorInFunction(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	expectedErr := errors.New("operation failed")

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Rollback").Return(nil)

	err := WithTransaction(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) error {
		return expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.False(t, mockTx.committed)
	assert.True(t, mockTx.rolledback)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransaction_BeginError(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	expectedErr := errors.New("failed to begin transaction")

	mockTxMgr.On("Begin", ctx).Return(nil, expectedErr)

	err := WithTransaction(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) error {
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to begin transaction")
	mockTxMgr.AssertExpectations(t)
}

func TestWithTransaction_CommitError(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	expectedErr := errors.New("commit failed")

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Commit").Return(expectedErr)

	err := WithTransaction(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) error {
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to commit transaction")
	assert.True(t, mockTx.committed)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransaction_RollbackError(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	operationErr := errors.New("operation failed")
	rollbackErr := errors.New("rollback failed")

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Rollback").Return(rollbackErr)

	err := WithTransaction(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) error {
		return operationErr
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction error")
	assert.Contains(t, err.Error(), "rollback error")
	assert.True(t, mockTx.rolledback)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransactionResult_Success(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	expectedResult := "success"

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Commit").Return(nil)

	result, err := WithTransactionResult(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) (string, error) {
		return expectedResult, nil
	})

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	assert.True(t, mockTx.committed)
	assert.False(t, mockTx.rolledback)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransactionResult_ErrorInFunction(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	expectedErr := errors.New("operation failed")

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Rollback").Return(nil)

	result, err := WithTransactionResult(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) (string, error) {
		return "", expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, "", result)
	assert.False(t, mockTx.committed)
	assert.True(t, mockTx.rolledback)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestWithTransactionResult_BeginError(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	expectedErr := errors.New("failed to begin transaction")

	mockTxMgr.On("Begin", ctx).Return(nil, expectedErr)

	result, err := WithTransactionResult(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) (int, error) {
		return 42, nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to begin transaction")
	assert.Equal(t, 0, result)
	mockTxMgr.AssertExpectations(t)
}

func TestWithTransactionResult_CommitError(t *testing.T) {
	ctx := context.Background()
	mockTxMgr := new(MockTransactionManager)
	mockTx := new(MockTransaction)
	expectedErr := errors.New("commit failed")

	mockTxMgr.On("Begin", ctx).Return(mockTx, nil)
	mockTx.On("Commit").Return(expectedErr)

	result, err := WithTransactionResult(ctx, mockTxMgr, func(ctx context.Context, tx repositories.Transaction) (int, error) {
		return 42, nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to commit transaction")
	assert.Equal(t, 42, result)
	assert.True(t, mockTx.committed)
	mockTxMgr.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

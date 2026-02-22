package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomainError(t *testing.T) {
	baseErr := errors.New("base error")
	domainErr := NewDomainError(ErrorTypeNotFound, "resource not found", baseErr)

	assert.Equal(t, ErrorTypeNotFound, domainErr.Type)
	assert.Equal(t, "resource not found", domainErr.Message)
	assert.Equal(t, baseErr, domainErr.Err)
	assert.NotNil(t, domainErr.Details)
}

func TestDomainError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *DomainError
		wantMsg string
	}{
		{
			name: "error with wrapped error",
			err: &DomainError{
				Type:    ErrorTypeNotFound,
				Message: "user not found",
				Err:     errors.New("db error"),
			},
			wantMsg: "not_found: user not found (db error)",
		},
		{
			name: "error without wrapped error",
			err: &DomainError{
				Type:    ErrorTypeValidation,
				Message: "invalid input",
				Err:     nil,
			},
			wantMsg: "validation: invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")
	domainErr := NewDomainError(ErrorTypeInternal, "internal error", baseErr)

	unwrapped := errors.Unwrap(domainErr)
	assert.Equal(t, baseErr, unwrapped)
}

func TestDomainError_Is(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "same error type",
			err:    NewDomainError(ErrorTypeNotFound, "not found", nil),
			target: ErrPolicyNotFound,
			want:   true,
		},
		{
			name:   "different error type",
			err:    NewDomainError(ErrorTypeValidation, "validation", nil),
			target: ErrPolicyNotFound,
			want:   false,
		},
		{
			name:   "not a domain error",
			err:    NewDomainError(ErrorTypeNotFound, "not found", nil),
			target: errors.New("regular error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, errors.Is(tt.err, tt.target))
		})
	}
}

func TestDomainError_WithDetail(t *testing.T) {
	err := NewDomainError(ErrorTypeValidation, "validation error", nil)
	
	err.WithDetail("field", "email").WithDetail("value", "invalid-email")

	assert.Equal(t, "email", err.Details["field"])
	assert.Equal(t, "invalid-email", err.Details["value"])
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"not found error", ErrPolicyNotFound, true},
		{"wrapped not found", fmt.Errorf("wrapped: %w", ErrUserNotFound), true},
		{"validation error", ErrInvalidInput, false},
		{"regular error", errors.New("regular"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNotFoundError(tt.err))
		})
	}
}

func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"validation error", ErrInvalidInput, true},
		{"wrapped validation", fmt.Errorf("wrapped: %w", ErrInvalidEmail), true},
		{"not found error", ErrPolicyNotFound, false},
		{"regular error", errors.New("regular"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidationError(tt.err))
		})
	}
}

func TestIsUnauthorizedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unauthorized error", ErrUnauthorized, true},
		{"invalid api key", ErrInvalidAPIKey, true},
		{"validation error", ErrInvalidInput, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsUnauthorizedError(tt.err))
		})
	}
}

func TestIsForbiddenError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"forbidden error", ErrForbidden, true},
		{"insufficient permissions", ErrInsufficientPermissions, true},
		{"unauthorized error", ErrUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsForbiddenError(tt.err))
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rate limit error", ErrRateLimitExceeded, true},
		{"requests per minute", ErrRequestsPerMinuteExceeded, true},
		{"tokens per hour", ErrTokensPerHourExceeded, true},
		{"budget error", ErrBudgetExceeded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRateLimitError(tt.err))
		})
	}
}

func TestIsBudgetError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"budget error", ErrBudgetExceeded, true},
		{"daily budget", ErrDailyBudgetExceeded, true},
		{"monthly budget", ErrMonthlyBudgetExceeded, true},
		{"rate limit error", ErrRateLimitExceeded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsBudgetError(tt.err))
		})
	}
}

func TestIsConflictError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"duplicate slug", ErrDuplicateSlug, true},
		{"duplicate email", ErrDuplicateEmail, true},
		{"validation error", ErrInvalidInput, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsConflictError(tt.err))
		})
	}
}

func TestIsInternalError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"internal error", ErrInternal, true},
		{"database error", ErrDatabaseError, true},
		{"external error", ErrProviderError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsInternalError(tt.err))
		})
	}
}

func TestIsExternalError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"provider unavailable", ErrProviderUnavailable, true},
		{"provider timeout", ErrProviderTimeout, true},
		{"internal error", ErrInternal, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsExternalError(tt.err))
		})
	}
}

func TestIsPolicyViolationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"policy violation", ErrPolicyViolation, true},
		{"PII detected", ErrPIIDetected, true},
		{"injection detected", ErrInjectionDetected, true},
		{"validation error", ErrInvalidInput, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPolicyViolationError(tt.err))
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{"not found", ErrPolicyNotFound, ErrorTypeNotFound},
		{"validation", ErrInvalidInput, ErrorTypeValidation},
		{"rate limit", ErrRateLimitExceeded, ErrorTypeRateLimit},
		{"budget", ErrBudgetExceeded, ErrorTypeBudget},
		{"regular error", errors.New("regular"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetErrorType(tt.err))
		})
	}
}

func TestGetErrorDetails(t *testing.T) {
	err := NewDomainError(ErrorTypeValidation, "validation error", nil)
	err.WithDetail("field", "email").WithDetail("reason", "invalid format")

	details := GetErrorDetails(err)
	require.NotNil(t, details)
	assert.Equal(t, "email", details["field"])
	assert.Equal(t, "invalid format", details["reason"])

	regularErr := errors.New("regular error")
	assert.Nil(t, GetErrorDetails(regularErr))
}

func TestWrapError(t *testing.T) {
	baseErr := errors.New("base error")
	wrapped := WrapError(ErrorTypeInternal, "wrapped message", baseErr)

	var domainErr *DomainError
	require.True(t, errors.As(wrapped, &domainErr))
	assert.Equal(t, ErrorTypeInternal, domainErr.Type)
	assert.Equal(t, "wrapped message", domainErr.Message)
	assert.Equal(t, baseErr, errors.Unwrap(wrapped))
}

func TestWrapInternal(t *testing.T) {
	baseErr := errors.New("database connection failed")
	wrapped := WrapInternal("failed to connect", baseErr)

	assert.True(t, IsInternalError(wrapped))
	assert.Equal(t, baseErr, errors.Unwrap(wrapped))
}

func TestWrapExternal(t *testing.T) {
	baseErr := errors.New("openai api error")
	wrapped := WrapExternal("provider request failed", baseErr)

	assert.True(t, IsExternalError(wrapped))
	assert.Equal(t, baseErr, errors.Unwrap(wrapped))
}

func TestAllErrorVariablesAreDefined(t *testing.T) {
	// Test that all predefined error variables are properly initialized
	errorVars := []error{
		// Not Found
		ErrPolicyNotFound,
		ErrOrganizationNotFound,
		ErrApplicationNotFound,
		ErrUserNotFound,
		ErrAuditLogNotFound,
		ErrRequestNotFound,
		
		// Validation
		ErrInvalidInput,
		ErrInvalidPolicyConfig,
		ErrInvalidModel,
		ErrInvalidProvider,
		ErrEmptyPrompt,
		ErrInvalidSlug,
		ErrInvalidEmail,
		
		// Authorization
		ErrUnauthorized,
		ErrInvalidAPIKey,
		ErrInvalidToken,
		ErrTokenExpired,
		
		// Permission
		ErrForbidden,
		ErrInsufficientPermissions,
		ErrOrgMismatch,
		
		// Rate Limit
		ErrRateLimitExceeded,
		ErrRequestsPerMinuteExceeded,
		ErrRequestsPerHourExceeded,
		ErrRequestsPerDayExceeded,
		ErrTokensPerMinuteExceeded,
		ErrTokensPerHourExceeded,
		ErrTokensPerDayExceeded,
		
		// Budget
		ErrBudgetExceeded,
		ErrDailyBudgetExceeded,
		ErrMonthlyBudgetExceeded,
		ErrCostPerRequestExceeded,
		
		// Conflict
		ErrDuplicateSlug,
		ErrDuplicateEmail,
		ErrDuplicateAPIKey,
		ErrConcurrentUpdate,
		
		// Internal
		ErrInternal,
		ErrDatabaseError,
		ErrTransactionFailed,
		ErrCacheFailed,
		
		// External
		ErrProviderUnavailable,
		ErrProviderTimeout,
		ErrProviderError,
		ErrProviderRateLimit,
		
		// Policy Violation
		ErrPolicyViolation,
		ErrPIIDetected,
		ErrInjectionDetected,
		ErrContentFiltered,
		ErrModelNotAllowed,
		ErrProviderNotAllowed,
	}

	for _, err := range errorVars {
		assert.NotNil(t, err, "error variable should not be nil")
		assert.NotEmpty(t, err.Error(), "error should have a message")
	}
}

func TestErrorTypeCheckersCoverage(t *testing.T) {
	// Ensure all error types have corresponding checker functions
	typeCheckers := map[ErrorType]func(error) bool{
		ErrorTypeNotFound:        IsNotFoundError,
		ErrorTypeValidation:      IsValidationError,
		ErrorTypeUnauthorized:    IsUnauthorizedError,
		ErrorTypeForbidden:       IsForbiddenError,
		ErrorTypeRateLimit:       IsRateLimitError,
		ErrorTypeBudget:          IsBudgetError,
		ErrorTypeConflict:        IsConflictError,
		ErrorTypeInternal:        IsInternalError,
		ErrorTypeExternal:        IsExternalError,
		ErrorTypePolicyViolation: IsPolicyViolationError,
	}

	for errType, checker := range typeCheckers {
		t.Run(string(errType), func(t *testing.T) {
			err := NewDomainError(errType, "test error", nil)
			assert.True(t, checker(err), "checker should return true for %s", errType)
		})
	}
}

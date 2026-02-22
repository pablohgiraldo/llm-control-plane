package services

import (
	"errors"
	"fmt"
)

// ErrorType represents the type/category of error
type ErrorType string

const (
	ErrorTypeNotFound      ErrorType = "not_found"
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeUnauthorized  ErrorType = "unauthorized"
	ErrorTypeForbidden     ErrorType = "forbidden"
	ErrorTypeRateLimit     ErrorType = "rate_limit"
	ErrorTypeBudget        ErrorType = "budget"
	ErrorTypeConflict      ErrorType = "conflict"
	ErrorTypeInternal      ErrorType = "internal"
	ErrorTypeExternal      ErrorType = "external"
	ErrorTypePolicyViolation ErrorType = "policy_violation"
)

// DomainError represents a structured error with additional context
type DomainError struct {
	Type    ErrorType
	Message string
	Err     error
	Details map[string]interface{}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap implements errors.Unwrap
func (e *DomainError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// WithDetail adds a detail to the error
func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// NewDomainError creates a new domain error
func NewDomainError(errType ErrorType, message string, err error) *DomainError {
	return &DomainError{
		Type:    errType,
		Message: message,
		Err:     err,
		Details: make(map[string]interface{}),
	}
}

// Domain error variables

var (
	// Not Found Errors
	ErrPolicyNotFound       = NewDomainError(ErrorTypeNotFound, "policy not found", nil)
	ErrOrganizationNotFound = NewDomainError(ErrorTypeNotFound, "organization not found", nil)
	ErrApplicationNotFound  = NewDomainError(ErrorTypeNotFound, "application not found", nil)
	ErrUserNotFound         = NewDomainError(ErrorTypeNotFound, "user not found", nil)
	ErrAuditLogNotFound     = NewDomainError(ErrorTypeNotFound, "audit log not found", nil)
	ErrRequestNotFound      = NewDomainError(ErrorTypeNotFound, "inference request not found", nil)

	// Validation Errors
	ErrInvalidInput          = NewDomainError(ErrorTypeValidation, "invalid input", nil)
	ErrInvalidPolicyConfig   = NewDomainError(ErrorTypeValidation, "invalid policy configuration", nil)
	ErrInvalidModel          = NewDomainError(ErrorTypeValidation, "invalid model specified", nil)
	ErrInvalidProvider       = NewDomainError(ErrorTypeValidation, "invalid provider specified", nil)
	ErrEmptyPrompt           = NewDomainError(ErrorTypeValidation, "prompt cannot be empty", nil)
	ErrInvalidSlug           = NewDomainError(ErrorTypeValidation, "invalid slug format", nil)
	ErrInvalidEmail          = NewDomainError(ErrorTypeValidation, "invalid email format", nil)

	// Authorization Errors
	ErrUnauthorized     = NewDomainError(ErrorTypeUnauthorized, "unauthorized", nil)
	ErrInvalidAPIKey    = NewDomainError(ErrorTypeUnauthorized, "invalid API key", nil)
	ErrInvalidToken     = NewDomainError(ErrorTypeUnauthorized, "invalid authentication token", nil)
	ErrTokenExpired     = NewDomainError(ErrorTypeUnauthorized, "authentication token expired", nil)

	// Permission Errors
	ErrForbidden             = NewDomainError(ErrorTypeForbidden, "access forbidden", nil)
	ErrInsufficientPermissions = NewDomainError(ErrorTypeForbidden, "insufficient permissions", nil)
	ErrOrgMismatch           = NewDomainError(ErrorTypeForbidden, "organization mismatch", nil)

	// Rate Limit Errors
	ErrRateLimitExceeded       = NewDomainError(ErrorTypeRateLimit, "rate limit exceeded", nil)
	ErrRequestsPerMinuteExceeded = NewDomainError(ErrorTypeRateLimit, "requests per minute limit exceeded", nil)
	ErrRequestsPerHourExceeded  = NewDomainError(ErrorTypeRateLimit, "requests per hour limit exceeded", nil)
	ErrRequestsPerDayExceeded   = NewDomainError(ErrorTypeRateLimit, "requests per day limit exceeded", nil)
	ErrTokensPerMinuteExceeded  = NewDomainError(ErrorTypeRateLimit, "tokens per minute limit exceeded", nil)
	ErrTokensPerHourExceeded    = NewDomainError(ErrorTypeRateLimit, "tokens per hour limit exceeded", nil)
	ErrTokensPerDayExceeded     = NewDomainError(ErrorTypeRateLimit, "tokens per day limit exceeded", nil)

	// Budget Errors
	ErrBudgetExceeded       = NewDomainError(ErrorTypeBudget, "budget exceeded", nil)
	ErrDailyBudgetExceeded  = NewDomainError(ErrorTypeBudget, "daily budget exceeded", nil)
	ErrMonthlyBudgetExceeded = NewDomainError(ErrorTypeBudget, "monthly budget exceeded", nil)
	ErrCostPerRequestExceeded = NewDomainError(ErrorTypeBudget, "cost per request limit exceeded", nil)

	// Conflict Errors
	ErrDuplicateSlug      = NewDomainError(ErrorTypeConflict, "slug already exists", nil)
	ErrDuplicateEmail     = NewDomainError(ErrorTypeConflict, "email already exists", nil)
	ErrDuplicateAPIKey    = NewDomainError(ErrorTypeConflict, "API key already exists", nil)
	ErrConcurrentUpdate   = NewDomainError(ErrorTypeConflict, "concurrent update detected", nil)

	// Internal Errors
	ErrInternal          = NewDomainError(ErrorTypeInternal, "internal server error", nil)
	ErrDatabaseError     = NewDomainError(ErrorTypeInternal, "database error", nil)
	ErrTransactionFailed = NewDomainError(ErrorTypeInternal, "transaction failed", nil)
	ErrCacheFailed       = NewDomainError(ErrorTypeInternal, "cache operation failed", nil)

	// External Provider Errors
	ErrProviderUnavailable = NewDomainError(ErrorTypeExternal, "LLM provider unavailable", nil)
	ErrProviderTimeout     = NewDomainError(ErrorTypeExternal, "LLM provider timeout", nil)
	ErrProviderError       = NewDomainError(ErrorTypeExternal, "LLM provider error", nil)
	ErrProviderRateLimit   = NewDomainError(ErrorTypeExternal, "LLM provider rate limit", nil)

	// Policy Violation Errors
	ErrPolicyViolation      = NewDomainError(ErrorTypePolicyViolation, "policy violation", nil)
	ErrPIIDetected          = NewDomainError(ErrorTypePolicyViolation, "PII detected in prompt", nil)
	ErrInjectionDetected    = NewDomainError(ErrorTypePolicyViolation, "prompt injection detected", nil)
	ErrContentFiltered      = NewDomainError(ErrorTypePolicyViolation, "content filtered by policy", nil)
	ErrModelNotAllowed      = NewDomainError(ErrorTypePolicyViolation, "model not allowed by policy", nil)
	ErrProviderNotAllowed   = NewDomainError(ErrorTypePolicyViolation, "provider not allowed by policy", nil)
)

// Error type checking helper functions

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeNotFound
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeValidation
	}
	return false
}

// IsUnauthorizedError checks if an error is an unauthorized error
func IsUnauthorizedError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeUnauthorized
	}
	return false
}

// IsForbiddenError checks if an error is a forbidden error
func IsForbiddenError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeForbidden
	}
	return false
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeRateLimit
	}
	return false
}

// IsBudgetError checks if an error is a budget error
func IsBudgetError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeBudget
	}
	return false
}

// IsConflictError checks if an error is a conflict error
func IsConflictError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeConflict
	}
	return false
}

// IsInternalError checks if an error is an internal error
func IsInternalError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeInternal
	}
	return false
}

// IsExternalError checks if an error is an external provider error
func IsExternalError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypeExternal
	}
	return false
}

// IsPolicyViolationError checks if an error is a policy violation error
func IsPolicyViolationError(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type == ErrorTypePolicyViolation
	}
	return false
}

// GetErrorType returns the ErrorType of a domain error, or empty string if not a domain error
func GetErrorType(err error) ErrorType {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type
	}
	return ""
}

// GetErrorDetails returns the details map of a domain error, or nil if not a domain error
func GetErrorDetails(err error) map[string]interface{} {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Details
	}
	return nil
}

// WrapError wraps an error with additional context
func WrapError(errType ErrorType, message string, err error) error {
	return NewDomainError(errType, message, err)
}

// WrapInternal wraps an error as an internal error
func WrapInternal(message string, err error) error {
	return NewDomainError(ErrorTypeInternal, message, err)
}

// WrapExternal wraps an error as an external provider error
func WrapExternal(message string, err error) error {
	return NewDomainError(ErrorTypeExternal, message, err)
}

package inference

import (
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/services/providers"
)

// CompletionRequest represents an inference request from the client
type CompletionRequest struct {
	// Authentication context
	OrgID  uuid.UUID  `json:"org_id"`
	AppID  uuid.UUID  `json:"app_id"`
	UserID *uuid.UUID `json:"user_id,omitempty"`

	// Model and provider
	Model    string  `json:"model"`
	Provider *string `json:"provider,omitempty"` // Optional - will be auto-selected if not specified

	// Messages for chat completion
	Messages []providers.Message `json:"messages"`

	// Model parameters
	MaxTokens        int     `json:"max_tokens,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"top_p,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty"`
	Stop             []string `json:"stop,omitempty"`

	// Request metadata
	RequestID string            `json:"request_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	IPAddress string            `json:"ip_address,omitempty"`
	UserAgent string            `json:"user_agent,omitempty"`

	// Options
	Stream bool `json:"stream,omitempty"`
}

// CompletionResponse represents the response from an inference request
type CompletionResponse struct {
	// Request tracking
	ID        uuid.UUID `json:"id"`
	RequestID string    `json:"request_id"`

	// Provider and model used
	Provider string `json:"provider"`
	Model    string `json:"model"`

	// Response content
	Choices []Choice `json:"choices"`

	// Usage statistics
	Usage Usage `json:"usage"`

	// Cost information
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`

	// Performance metrics
	LatencyMs int `json:"latency_ms"`

	// Timestamps
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`

	// Policy information
	PoliciesApplied []uuid.UUID `json:"policies_applied,omitempty"`

	// Metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int              `json:"index"`
	Message      providers.Message `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	ID        string    `json:"id"`
	RequestID string    `json:"request_id"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Choices   []Choice  `json:"choices"`
	Created   time.Time `json:"created"`
	Done      bool      `json:"done"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// InferenceError represents an error during inference
type InferenceError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int               `json:"status_code"`
	Retryable  bool              `json:"retryable"`
}

// Error implements the error interface
func (e *InferenceError) Error() string {
	return e.Message
}

// Common error codes
const (
	ErrCodeValidation       = "VALIDATION_ERROR"
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrCodeBudgetExceeded   = "BUDGET_EXCEEDED"
	ErrCodePolicyViolation  = "POLICY_VIOLATION"
	ErrCodeProviderError    = "PROVIDER_ERROR"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeInternal         = "INTERNAL_ERROR"
)

// NewValidationError creates a validation error
func NewValidationError(message string, details map[string]interface{}) *InferenceError {
	return &InferenceError{
		Code:       ErrCodeValidation,
		Message:    message,
		Details:    details,
		StatusCode: 400,
		Retryable:  false,
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(message string, details map[string]interface{}) *InferenceError {
	return &InferenceError{
		Code:       ErrCodeRateLimitExceeded,
		Message:    message,
		Details:    details,
		StatusCode: 429,
		Retryable:  true,
	}
}

// NewBudgetError creates a budget exceeded error
func NewBudgetError(message string, details map[string]interface{}) *InferenceError {
	return &InferenceError{
		Code:       ErrCodeBudgetExceeded,
		Message:    message,
		Details:    details,
		StatusCode: 402,
		Retryable:  false,
	}
}

// NewPolicyViolationError creates a policy violation error
func NewPolicyViolationError(message string, details map[string]interface{}) *InferenceError {
	return &InferenceError{
		Code:       ErrCodePolicyViolation,
		Message:    message,
		Details:    details,
		StatusCode: 403,
		Retryable:  false,
	}
}

// NewProviderError creates a provider error
func NewProviderError(message string, details map[string]interface{}, retryable bool) *InferenceError {
	statusCode := 502
	if !retryable {
		statusCode = 500
	}
	return &InferenceError{
		Code:       ErrCodeProviderError,
		Message:    message,
		Details:    details,
		StatusCode: statusCode,
		Retryable:  retryable,
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string, details map[string]interface{}) *InferenceError {
	return &InferenceError{
		Code:       ErrCodeInternal,
		Message:    message,
		Details:    details,
		StatusCode: 500,
		Retryable:  false,
	}
}

// PipelineContext holds context for the inference pipeline
type PipelineContext struct {
	Request       *CompletionRequest
	InferenceID   uuid.UUID
	StartTime     time.Time
	
	// Policy evaluation results
	PolicyResult  interface{}
	AppliedPolicies []uuid.UUID
	
	// Rate limiting
	RateLimitPassed bool
	
	// Budget
	EstimatedCost   float64
	BudgetPassed    bool
	
	// Prompt validation
	PromptValidated bool
	SanitizedPrompt string
	
	// Routing
	SelectedProvider string
	
	// Provider response
	ProviderResponse *providers.ChatResponse
	
	// Final cost
	ActualCost      float64
}

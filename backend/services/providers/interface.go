package providers

import (
	"context"
	"time"
)

// Provider represents a unified LLM provider interface
type Provider interface {
	// Name returns the provider name (e.g., "openai", "anthropic", "azure")
	Name() string

	// ChatCompletion performs a chat completion request
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// IsAvailable checks if the provider is currently available
	IsAvailable(ctx context.Context) bool

	// EstimateCost estimates the cost for a given request
	EstimateCost(req *ChatRequest) (float64, error)

	// ValidateModel checks if a model is supported by this provider
	ValidateModel(model string) error

	// GetModelInfo returns information about a specific model
	GetModelInfo(model string) (*ModelInfo, error)

	// ListModels returns all available models from this provider
	ListModels() []string
}

// ChatRequest represents a unified chat completion request
type ChatRequest struct {
	// Model identifier (e.g., "gpt-4", "claude-3-opus")
	Model string `json:"model"`

	// Messages in the conversation
	Messages []Message `json:"messages"`

	// MaxTokens limits the response length
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0 to 2.0)
	Temperature float64 `json:"temperature,omitempty"`

	// TopP controls nucleus sampling
	TopP float64 `json:"top_p,omitempty"`

	// Stream enables streaming responses
	Stream bool `json:"stream,omitempty"`

	// Stop sequences
	Stop []string `json:"stop,omitempty"`

	// FrequencyPenalty reduces repetition (-2.0 to 2.0)
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`

	// PresencePenalty encourages new topics (-2.0 to 2.0)
	PresencePenalty float64 `json:"presence_penalty,omitempty"`

	// User identifier for abuse monitoring
	User string `json:"user,omitempty"`

	// Metadata for tracking and logging
	Metadata map[string]string `json:"metadata,omitempty"`

	// Timeout for the request
	Timeout time.Duration `json:"-"`
}

// Message represents a single message in a conversation
type Message struct {
	// Role can be "system", "user", or "assistant"
	Role string `json:"role"`

	// Content is the message text
	Content string `json:"content"`

	// Name is an optional identifier for the message sender
	Name string `json:"name,omitempty"`

	// FunctionCall is used for function calling (provider-specific)
	FunctionCall interface{} `json:"function_call,omitempty"`
}

// ChatResponse represents a unified chat completion response
type ChatResponse struct {
	// ID is the unique identifier for this completion
	ID string `json:"id"`

	// Model used for the completion
	Model string `json:"model"`

	// Choices contains the completion results
	Choices []Choice `json:"choices"`

	// Usage statistics
	Usage Usage `json:"usage"`

	// Provider that handled the request
	Provider string `json:"provider"`

	// Latency of the request
	Latency time.Duration `json:"latency"`

	// Created timestamp
	Created time.Time `json:"created"`

	// Metadata from the request
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Choice represents a completion choice
type Choice struct {
	// Index of this choice
	Index int `json:"index"`

	// Message contains the response
	Message Message `json:"message"`

	// FinishReason indicates why the completion finished
	// Values: "stop", "length", "content_filter", "function_call"
	FinishReason string `json:"finish_reason"`

	// LogProbs contains token log probabilities (if requested)
	LogProbs interface{} `json:"logprobs,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	// PromptTokens used in the request
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens used in the response
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the sum of prompt and completion tokens
	TotalTokens int `json:"total_tokens"`
}

// ModelInfo contains metadata about a model
type ModelInfo struct {
	// ID is the model identifier
	ID string `json:"id"`

	// Name is the human-readable name
	Name string `json:"name"`

	// Provider that offers this model
	Provider string `json:"provider"`

	// Description of the model
	Description string `json:"description"`

	// MaxTokens supported by the model
	MaxTokens int `json:"max_tokens"`

	// ContextWindow size
	ContextWindow int `json:"context_window"`

	// Pricing information
	PricingPerPromptToken     float64 `json:"pricing_per_prompt_token"`
	PricingPerCompletionToken float64 `json:"pricing_per_completion_token"`

	// Capabilities
	SupportsStreaming   bool `json:"supports_streaming"`
	SupportsFunctions   bool `json:"supports_functions"`
	SupportsVision      bool `json:"supports_vision"`
	SupportsJSON        bool `json:"supports_json"`

	// Deprecated indicates if the model is deprecated
	Deprecated bool `json:"deprecated"`
}

// ProviderConfig holds common configuration for providers
type ProviderConfig struct {
	// APIKey for authentication
	APIKey string

	// BaseURL for the API (optional override)
	BaseURL string

	// Timeout for requests
	Timeout time.Duration

	// MaxRetries for failed requests
	MaxRetries int

	// RetryDelay between retries
	RetryDelay time.Duration

	// Additional headers
	Headers map[string]string

	// OrgID for organization-specific endpoints
	OrgID string

	// EnableMetrics enables metrics collection
	EnableMetrics bool
}

// DefaultProviderConfig returns a sensible default configuration
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		Headers:    make(map[string]string),
		EnableMetrics: true,
	}
}

// ProviderError represents an error from a provider
type ProviderError struct {
	// Provider that generated the error
	Provider string

	// Code is the error code
	Code string

	// Message is the error message
	Message string

	// StatusCode is the HTTP status code (if applicable)
	StatusCode int

	// Retryable indicates if the request can be retried
	Retryable bool

	// Cause is the underlying error
	Cause error
}

// Error implements the error interface
func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap implements error unwrapping
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a new provider error
func NewProviderError(provider, code, message string, statusCode int, retryable bool, cause error) *ProviderError {
	return &ProviderError{
		Provider:   provider,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  retryable,
		Cause:      cause,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if provErr, ok := err.(*ProviderError); ok {
		return provErr.Retryable
	}
	return false
}

// StreamCallback is called for each chunk in a streaming response
type StreamCallback func(chunk *ChatResponse) error

// StreamingProvider extends Provider with streaming support
type StreamingProvider interface {
	Provider

	// ChatCompletionStream performs a streaming chat completion
	ChatCompletionStream(ctx context.Context, req *ChatRequest, callback StreamCallback) error
}

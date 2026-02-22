package inference

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/upb/llm-control-plane/backend/services/providers"
	"go.uber.org/zap"
)

func TestNewInferenceService(t *testing.T) {
	logger := zap.NewNop()
	
	service := &InferenceService{
		logger: logger,
	}

	assert.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestCreateTestRequest(t *testing.T) {
	req := &CompletionRequest{
		OrgID:  uuid.New(),
		AppID:  uuid.New(),
		UserID: nil,
		Model:  "gpt-3.5-turbo",
		Messages: []providers.Message{
			{Role: "user", Content: "Hello, world!"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		RequestID:   uuid.New().String(),
	}

	assert.NotNil(t, req)
	assert.NotEqual(t, uuid.Nil, req.OrgID)
	assert.NotEqual(t, uuid.Nil, req.AppID)
	assert.Equal(t, "gpt-3.5-turbo", req.Model)
	assert.Len(t, req.Messages, 1)
}

func TestEstimatePromptTokens(t *testing.T) {
	service := &InferenceService{}

	tests := []struct {
		name     string
		messages []providers.Message
		expected int
	}{
		{
			name: "single short message",
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			expected: 1, // 5 chars / 4 = 1.25 ~ 1
		},
		{
			name: "multiple messages",
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			expected: 3, // 14 chars / 4 = 3.5 ~ 3
		},
		{
			name: "long message",
			messages: []providers.Message{
				{Role: "user", Content: "This is a longer message with more content to test token estimation"},
			},
			expected: 16, // 67 chars / 4 = 16.75 ~ 16
		},
		{
			name:     "empty messages",
			messages: []providers.Message{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := service.estimatePromptTokens(tt.messages)
			assert.Equal(t, tt.expected, tokens)
		})
	}
}

func TestCombineMessages(t *testing.T) {
	service := &InferenceService{}

	tests := []struct {
		name     string
		messages []providers.Message
		expected string
	}{
		{
			name: "single message",
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "multiple messages",
			messages: []providers.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			expected: "Hello\nHi there!\nHow are you?",
		},
		{
			name:     "empty messages",
			messages: []providers.Message{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combined := service.combineMessages(tt.messages)
			assert.Equal(t, tt.expected, combined)
		})
	}
}

func TestGetRequestedProvider(t *testing.T) {
	service := &InferenceService{}

	t.Run("with provider specified", func(t *testing.T) {
		provider := "openai"
		req := &CompletionRequest{
			Provider: &provider,
		}
		assert.Equal(t, "openai", service.getRequestedProvider(req))
	})

	t.Run("without provider specified", func(t *testing.T) {
		req := &CompletionRequest{}
		assert.Equal(t, "", service.getRequestedProvider(req))
	})
}

func TestEstimateCostForTokens(t *testing.T) {
	service := &InferenceService{}

	tests := []struct {
		name   string
		model  string
		tokens int
		minCost float64
	}{
		{
			name:   "gpt-4",
			model:  "gpt-4",
			tokens: 1000,
			minCost: 0.02,
		},
		{
			name:   "gpt-3.5-turbo",
			model:  "gpt-3.5-turbo",
			tokens: 1000,
			minCost: 0.0005,
		},
		{
			name:   "unknown model",
			model:  "unknown-model",
			tokens: 1000,
			minCost: 0.005,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := service.estimateCostForTokens(tt.model, tt.tokens)
			assert.Greater(t, cost, 0.0)
			assert.GreaterOrEqual(t, cost, tt.minCost*0.5) // Allow some variance
		})
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("test error", map[string]interface{}{
		"field": "model",
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "test error", err.Message)
	assert.Equal(t, 400, err.StatusCode)
	assert.False(t, err.Retryable)
	assert.Contains(t, err.Details, "field")
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError("rate limit exceeded", map[string]interface{}{
		"limit": 100,
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeRateLimitExceeded, err.Code)
	assert.Equal(t, 429, err.StatusCode)
	assert.True(t, err.Retryable)
}

func TestNewBudgetError(t *testing.T) {
	err := NewBudgetError("budget exceeded", map[string]interface{}{
		"limit": 1000.0,
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeBudgetExceeded, err.Code)
	assert.Equal(t, 402, err.StatusCode)
	assert.False(t, err.Retryable)
}

func TestNewPolicyViolationError(t *testing.T) {
	err := NewPolicyViolationError("policy violation", map[string]interface{}{
		"policy": "pii_detection",
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodePolicyViolation, err.Code)
	assert.Equal(t, 403, err.StatusCode)
	assert.False(t, err.Retryable)
}

func TestNewProviderError(t *testing.T) {
	t.Run("retryable", func(t *testing.T) {
		err := NewProviderError("provider timeout", nil, true)
		assert.NotNil(t, err)
		assert.Equal(t, ErrCodeProviderError, err.Code)
		assert.Equal(t, 502, err.StatusCode)
		assert.True(t, err.Retryable)
	})

	t.Run("not retryable", func(t *testing.T) {
		err := NewProviderError("provider error", nil, false)
		assert.NotNil(t, err)
		assert.Equal(t, ErrCodeProviderError, err.Code)
		assert.Equal(t, 500, err.StatusCode)
		assert.False(t, err.Retryable)
	})
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError("internal error", map[string]interface{}{
		"detail": "database connection failed",
	})

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, 500, err.StatusCode)
	assert.False(t, err.Retryable)
}

func TestInferenceError_Error(t *testing.T) {
	err := &InferenceError{
		Code:    ErrCodeValidation,
		Message: "test error message",
	}

	assert.Equal(t, "test error message", err.Error())
}

func TestPipelineContext(t *testing.T) {
	ctx := &PipelineContext{
		InferenceID: uuid.New(),
		Request: &CompletionRequest{
			OrgID: uuid.New(),
			AppID: uuid.New(),
			Model: "gpt-3.5-turbo",
		},
		AppliedPolicies: []uuid.UUID{uuid.New()},
		EstimatedCost:   0.001,
		ActualCost:      0.0015,
	}

	assert.NotEqual(t, uuid.Nil, ctx.InferenceID)
	assert.NotNil(t, ctx.Request)
	assert.Len(t, ctx.AppliedPolicies, 1)
	assert.Greater(t, ctx.EstimatedCost, 0.0)
	assert.Greater(t, ctx.ActualCost, 0.0)
}

func TestCompletionResponse(t *testing.T) {
	resp := &CompletionResponse{
		ID:        uuid.New(),
		RequestID: "test-request-123",
		Provider:  "openai",
		Model:     "gpt-3.5-turbo",
		Choices: []Choice{
			{
				Index: 0,
				Message: providers.Message{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		Cost:     0.0001,
		Currency: "USD",
	}

	assert.NotEqual(t, uuid.Nil, resp.ID)
	assert.Equal(t, "test-request-123", resp.RequestID)
	assert.Equal(t, "openai", resp.Provider)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
	assert.Greater(t, resp.Cost, 0.0)
}

func TestCreateInferenceRequest(t *testing.T) {
	service := &InferenceService{
		logger: zap.NewNop(),
	}

	req := &CompletionRequest{
		OrgID:  uuid.New(),
		AppID:  uuid.New(),
		UserID: nil,
		Model:  "gpt-3.5-turbo",
		Messages: []providers.Message{
			{Role: "user", Content: "Test message"},
		},
		RequestID: "test-request-123",
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	inferenceID := uuid.New()
	inferenceReq := service.createInferenceRequest(req, inferenceID)

	assert.NotNil(t, inferenceReq)
	assert.Equal(t, inferenceID, inferenceReq.ID)
	assert.Equal(t, req.OrgID, inferenceReq.OrgID)
	assert.Equal(t, req.AppID, inferenceReq.AppID)
	assert.Equal(t, req.Model, inferenceReq.Model)
	assert.Equal(t, req.RequestID, inferenceReq.RequestID)
	assert.Equal(t, req.IPAddress, inferenceReq.IPAddress)
	assert.Equal(t, req.UserAgent, inferenceReq.UserAgent)
}

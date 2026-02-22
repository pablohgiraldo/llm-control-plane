package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockProvider is a test implementation of the Provider interface
type MockProvider struct {
	name          string
	available     bool
	models        []string
	estimateCost  float64
	validateError error
	responseDelay time.Duration
}

// Helper methods for testing
func (m *MockProvider) SetModels(models []string) {
	m.models = models
}

func (m *MockProvider) SetAvailable(available bool) {
	m.available = available
}

func (m *MockProvider) SetEstimateCost(cost float64) {
	m.estimateCost = cost
}

func (m *MockProvider) SetResponseDelay(delay time.Duration) {
	m.responseDelay = delay
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:      name,
		available: true,
		models:    []string{"mock-model-1", "mock-model-2"},
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if m.responseDelay > 0 {
		select {
		case <-time.After(m.responseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &ChatResponse{
		ID:       "mock-response-123",
		Model:    req.Model,
		Provider: m.name,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "This is a mock response",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Latency: m.responseDelay,
		Created: time.Now(),
	}, nil
}

func (m *MockProvider) IsAvailable(ctx context.Context) bool {
	return m.available
}

func (m *MockProvider) EstimateCost(req *ChatRequest) (float64, error) {
	if m.estimateCost > 0 {
		return m.estimateCost, nil
	}
	return 0.001, nil
}

func (m *MockProvider) ValidateModel(model string) error {
	if m.validateError != nil {
		return m.validateError
	}
	for _, m := range m.models {
		if m == model {
			return nil
		}
	}
	return errors.New("model not supported")
}

func (m *MockProvider) GetModelInfo(model string) (*ModelInfo, error) {
	if err := m.ValidateModel(model); err != nil {
		return nil, err
	}
	return &ModelInfo{
		ID:                        model,
		Name:                      model,
		Provider:                  m.name,
		MaxTokens:                 4096,
		ContextWindow:             8192,
		PricingPerPromptToken:     0.00001,
		PricingPerCompletionToken: 0.00002,
		SupportsStreaming:         true,
		SupportsFunctions:         true,
	}, nil
}

func (m *MockProvider) ListModels() []string {
	return m.models
}

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("test-provider")

	t.Run("Name", func(t *testing.T) {
		if provider.Name() != "test-provider" {
			t.Errorf("Name() = %s, want test-provider", provider.Name())
		}
	})

	t.Run("IsAvailable", func(t *testing.T) {
		ctx := context.Background()
		if !provider.IsAvailable(ctx) {
			t.Error("IsAvailable() = false, want true")
		}

		provider.available = false
		if provider.IsAvailable(ctx) {
			t.Error("IsAvailable() = true, want false")
		}
	})

	t.Run("ChatCompletion", func(t *testing.T) {
		ctx := context.Background()
		req := &ChatRequest{
			Model: "mock-model-1",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		resp, err := provider.ChatCompletion(ctx, req)
		if err != nil {
			t.Fatalf("ChatCompletion() error = %v", err)
		}

		if resp.ID == "" {
			t.Error("Response ID is empty")
		}

		if len(resp.Choices) == 0 {
			t.Error("Response has no choices")
		}

		if resp.Usage.TotalTokens == 0 {
			t.Error("Usage tokens not set")
		}
	})

	t.Run("EstimateCost", func(t *testing.T) {
		req := &ChatRequest{
			Model:    "mock-model-1",
			Messages: []Message{{Role: "user", Content: "test"}},
		}

		cost, err := provider.EstimateCost(req)
		if err != nil {
			t.Fatalf("EstimateCost() error = %v", err)
		}

		if cost <= 0 {
			t.Errorf("EstimateCost() = %f, want > 0", cost)
		}
	})

	t.Run("ValidateModel", func(t *testing.T) {
		err := provider.ValidateModel("mock-model-1")
		if err != nil {
			t.Errorf("ValidateModel() error = %v for valid model", err)
		}

		err = provider.ValidateModel("invalid-model")
		if err == nil {
			t.Error("ValidateModel() expected error for invalid model")
		}
	})

	t.Run("GetModelInfo", func(t *testing.T) {
		info, err := provider.GetModelInfo("mock-model-1")
		if err != nil {
			t.Fatalf("GetModelInfo() error = %v", err)
		}

		if info.ID != "mock-model-1" {
			t.Errorf("GetModelInfo() ID = %s, want mock-model-1", info.ID)
		}

		if info.MaxTokens == 0 {
			t.Error("GetModelInfo() MaxTokens not set")
		}
	})

	t.Run("ListModels", func(t *testing.T) {
		models := provider.ListModels()
		if len(models) != 2 {
			t.Errorf("ListModels() returned %d models, want 2", len(models))
		}
	})
}

func TestChatRequest(t *testing.T) {
	req := &ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		TopP:        0.9,
		Stream:      false,
		Stop:        []string{"\n"},
		User:        "user-123",
		Metadata:    map[string]string{"session": "abc"},
		Timeout:     30 * time.Second,
	}

	if req.Model != "test-model" {
		t.Errorf("Model = %s, want test-model", req.Model)
	}

	if len(req.Messages) != 2 {
		t.Errorf("len(Messages) = %d, want 2", len(req.Messages))
	}

	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Temperature)
	}
}

func TestChatResponse(t *testing.T) {
	resp := &ChatResponse{
		ID:       "resp-123",
		Model:    "test-model",
		Provider: "test-provider",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
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
		Latency: 200 * time.Millisecond,
		Created: time.Now(),
	}

	if resp.ID != "resp-123" {
		t.Errorf("ID = %s, want resp-123", resp.ID)
	}

	if len(resp.Choices) != 1 {
		t.Errorf("len(Choices) = %d, want 1", len(resp.Choices))
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestModelInfo(t *testing.T) {
	info := &ModelInfo{
		ID:                        "test-model-1",
		Name:                      "Test Model 1",
		Provider:                  "test-provider",
		Description:               "A test model",
		MaxTokens:                 4096,
		ContextWindow:             8192,
		PricingPerPromptToken:     0.00001,
		PricingPerCompletionToken: 0.00002,
		SupportsStreaming:         true,
		SupportsFunctions:         true,
		SupportsVision:            false,
		SupportsJSON:              true,
		Deprecated:                false,
	}

	if info.ID != "test-model-1" {
		t.Errorf("ID = %s, want test-model-1", info.ID)
	}

	if info.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", info.MaxTokens)
	}

	if !info.SupportsStreaming {
		t.Error("SupportsStreaming should be true")
	}
}

func TestProviderConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultProviderConfig()

		if config.Timeout == 0 {
			t.Error("Default timeout not set")
		}

		if config.MaxRetries == 0 {
			t.Error("Default max retries not set")
		}

		if config.Headers == nil {
			t.Error("Headers map not initialized")
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := ProviderConfig{
			APIKey:     "test-key",
			BaseURL:    "https://api.test.com",
			Timeout:    60 * time.Second,
			MaxRetries: 5,
			RetryDelay: 2 * time.Second,
			Headers: map[string]string{
				"X-Custom": "value",
			},
			OrgID:         "org-123",
			EnableMetrics: true,
		}

		if config.APIKey != "test-key" {
			t.Errorf("APIKey = %s, want test-key", config.APIKey)
		}

		if config.MaxRetries != 5 {
			t.Errorf("MaxRetries = %d, want 5", config.MaxRetries)
		}

		if config.Headers["X-Custom"] != "value" {
			t.Error("Custom header not set")
		}
	})
}

func TestProviderError(t *testing.T) {
	t.Run("NewProviderError", func(t *testing.T) {
		cause := errors.New("connection failed")
		err := NewProviderError(
			"test-provider",
			"CONN_ERROR",
			"Failed to connect",
			500,
			true,
			cause,
		)

		if err.Provider != "test-provider" {
			t.Errorf("Provider = %s, want test-provider", err.Provider)
		}

		if err.Code != "CONN_ERROR" {
			t.Errorf("Code = %s, want CONN_ERROR", err.Code)
		}

		if err.StatusCode != 500 {
			t.Errorf("StatusCode = %d, want 500", err.StatusCode)
		}

		if !err.Retryable {
			t.Error("Error should be retryable")
		}

		if err.Cause != cause {
			t.Error("Cause not set correctly")
		}
	})

	t.Run("ErrorMethod", func(t *testing.T) {
		err := NewProviderError("provider", "CODE", "message", 400, false, nil)
		if err.Error() != "message" {
			t.Errorf("Error() = %s, want message", err.Error())
		}

		cause := errors.New("cause")
		err = NewProviderError("provider", "CODE", "message", 400, false, cause)
		if err.Error() != "message: cause" {
			t.Errorf("Error() = %s, want 'message: cause'", err.Error())
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := NewProviderError("provider", "CODE", "message", 500, true, cause)

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Error("Unwrap() did not return the correct cause")
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		retryableErr := NewProviderError("provider", "CODE", "message", 500, true, nil)
		if !IsRetryable(retryableErr) {
			t.Error("IsRetryable() = false, want true")
		}

		nonRetryableErr := NewProviderError("provider", "CODE", "message", 400, false, nil)
		if IsRetryable(nonRetryableErr) {
			t.Error("IsRetryable() = true, want false")
		}

		standardErr := errors.New("standard error")
		if IsRetryable(standardErr) {
			t.Error("IsRetryable() should return false for non-ProviderError")
		}
	})
}

func TestContextCancellation(t *testing.T) {
	provider := NewMockProvider("test")
	provider.responseDelay = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &ChatRequest{
		Model:    "mock-model-1",
		Messages: []Message{{Role: "user", Content: "test"}},
	}

	_, err := provider.ChatCompletion(ctx, req)
	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
		Name:    "Alice",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %s, want user", msg.Role)
	}

	if msg.Content != "Hello, world!" {
		t.Errorf("Content = %s, want 'Hello, world!'", msg.Content)
	}

	if msg.Name != "Alice" {
		t.Errorf("Name = %s, want Alice", msg.Name)
	}
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", usage.PromptTokens)
	}

	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want 50", usage.CompletionTokens)
	}

	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", usage.TotalTokens)
	}

	// Verify sum
	if usage.PromptTokens+usage.CompletionTokens != usage.TotalTokens {
		t.Error("TotalTokens should equal PromptTokens + CompletionTokens")
	}
}

func BenchmarkChatCompletion(b *testing.B) {
	provider := NewMockProvider("test")
	ctx := context.Background()
	req := &ChatRequest{
		Model:    "mock-model-1",
		Messages: []Message{{Role: "user", Content: "test"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.ChatCompletion(ctx, req)
	}
}

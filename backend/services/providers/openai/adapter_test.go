package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/upb/llm-control-plane/backend/services/providers"
)

func TestNewOpenAIAdapter(t *testing.T) {
	config := providers.ProviderConfig{
		APIKey: "test-key",
	}

	adapter := NewOpenAIAdapter(config)

	if adapter == nil {
		t.Fatal("NewOpenAIAdapter() returned nil")
	}

	if adapter.Name() != "openai" {
		t.Errorf("Name() = %s, want openai", adapter.Name())
	}

	if adapter.config.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %s, want %s", adapter.config.BaseURL, defaultBaseURL)
	}

	if len(adapter.models) == 0 {
		t.Error("Models not initialized")
	}
}

func TestOpenAIAdapter_ValidateModel(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	tests := []struct {
		name        string
		model       string
		expectError bool
	}{
		{
			name:        "valid model gpt-4",
			model:       "gpt-4",
			expectError: false,
		},
		{
			name:        "valid model gpt-3.5-turbo",
			model:       "gpt-3.5-turbo",
			expectError: false,
		},
		{
			name:        "valid model gpt-4-turbo",
			model:       "gpt-4-turbo",
			expectError: false,
		},
		{
			name:        "invalid model",
			model:       "invalid-model",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.ValidateModel(tt.model)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestOpenAIAdapter_GetModelInfo(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	tests := []struct {
		name        string
		model       string
		expectError bool
	}{
		{
			name:        "valid model",
			model:       "gpt-4",
			expectError: false,
		},
		{
			name:        "invalid model",
			model:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := adapter.GetModelInfo(tt.model)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if info.ID != tt.model {
					t.Errorf("Model ID = %s, want %s", info.ID, tt.model)
				}

				if info.Provider != "openai" {
					t.Errorf("Provider = %s, want openai", info.Provider)
				}

				if info.MaxTokens == 0 {
					t.Error("MaxTokens not set")
				}
			}
		})
	}
}

func TestOpenAIAdapter_ListModels(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	models := adapter.ListModels()

	if len(models) == 0 {
		t.Error("ListModels() returned empty list")
	}

	// Check for expected models
	expectedModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o", "gpt-4o-mini"}
	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s not found in list", expected)
		}
	}
}

func TestOpenAIAdapter_EstimateCost(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	tests := []struct {
		name      string
		request   *providers.ChatRequest
		wantError bool
	}{
		{
			name: "gpt-4 request",
			request: &providers.ChatRequest{
				Model: "gpt-4",
				Messages: []providers.Message{
					{Role: "user", Content: "Hello, how are you?"},
				},
				MaxTokens: 100,
			},
			wantError: false,
		},
		{
			name: "gpt-3.5-turbo request",
			request: &providers.ChatRequest{
				Model: "gpt-3.5-turbo",
				Messages: []providers.Message{
					{Role: "user", Content: "Tell me a joke"},
				},
				MaxTokens: 50,
			},
			wantError: false,
		},
		{
			name: "invalid model",
			request: &providers.ChatRequest{
				Model: "invalid-model",
				Messages: []providers.Message{
					{Role: "user", Content: "test"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := adapter.EstimateCost(tt.request)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if cost <= 0 {
					t.Errorf("Cost = %f, want > 0", cost)
				}

				t.Logf("Estimated cost for %s: $%.6f", tt.request.Model, cost)
			}
		})
	}
}

func TestOpenAIAdapter_ChatCompletion(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected path /chat/completions, got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Error("Authorization header missing or invalid")
		}

		// Read and parse request
		body, _ := io.ReadAll(r.Body)
		var req OpenAIChatRequest
		json.Unmarshal(body, &req)

		// Send mock response
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-test123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "This is a test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create adapter with mock server
	config := providers.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	}
	adapter := NewOpenAIAdapter(config)

	// Test request
	req := &providers.ChatRequest{
		Model: "gpt-4",
		Messages: []providers.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	ctx := context.Background()
	resp, err := adapter.ChatCompletion(ctx, req)

	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if resp.ID == "" {
		t.Error("Response ID is empty")
	}

	if resp.Provider != "openai" {
		t.Errorf("Provider = %s, want openai", resp.Provider)
	}

	if len(resp.Choices) == 0 {
		t.Error("No choices in response")
	}

	if resp.Choices[0].Message.Content != "This is a test response" {
		t.Errorf("Unexpected response content: %s", resp.Choices[0].Message.Content)
	}

	if resp.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", resp.Usage.TotalTokens)
	}
}

func TestOpenAIAdapter_ChatCompletion_Error(t *testing.T) {
	// Create mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		
		errResp := OpenAIErrorResponse{
			Error: OpenAIError{
				Message: "Invalid request",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		}
		json.NewEncoder(w).Encode(errResp)
	}))
	defer server.Close()

	config := providers.ProviderConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	}
	adapter := NewOpenAIAdapter(config)

	req := &providers.ChatRequest{
		Model: "gpt-4",
		Messages: []providers.Message{
			{Role: "user", Content: "test"},
		},
	}

	ctx := context.Background()
	_, err := adapter.ChatCompletion(ctx, req)

	if err == nil {
		t.Fatal("Expected error but got none")
	}

	provErr, ok := err.(*providers.ProviderError)
	if !ok {
		t.Fatalf("Expected ProviderError, got %T", err)
	}

	if provErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", provErr.StatusCode, http.StatusBadRequest)
	}
}

func TestOpenAIAdapter_ChatCompletion_Retry(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		// Fail first 2 attempts, succeed on 3rd
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Success response
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-test123",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Success after retry",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 10,
				TotalTokens:      20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := providers.ProviderConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	}
	adapter := NewOpenAIAdapter(config)

	req := &providers.ChatRequest{
		Model:    "gpt-4",
		Messages: []providers.Message{{Role: "user", Content: "test"}},
	}

	ctx := context.Background()
	resp, err := adapter.ChatCompletion(ctx, req)

	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestOpenAIAdapter_IsAvailable(t *testing.T) {
	t.Run("available", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": []}`))
		}))
		defer server.Close()

		config := providers.ProviderConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		}
		adapter := NewOpenAIAdapter(config)

		ctx := context.Background()
		available := adapter.IsAvailable(ctx)

		if !available {
			t.Error("Expected provider to be available")
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := providers.ProviderConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
		}
		adapter := NewOpenAIAdapter(config)

		ctx := context.Background()
		available := adapter.IsAvailable(ctx)

		if available {
			t.Error("Expected provider to be unavailable")
		}
	})
}

func TestBuildOpenAIRequest(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	maxTokens := 100
	temperature := 0.7
	topP := 0.9
	user := "test-user"

	req := &providers.ChatRequest{
		Model: "gpt-4",
		Messages: []providers.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello", Name: "Alice"},
		},
		MaxTokens:        maxTokens,
		Temperature:      temperature,
		TopP:             topP,
		Stream:           false,
		Stop:             []string{"\n"},
		FrequencyPenalty: 0.5,
		PresencePenalty:  0.3,
		User:             user,
	}

	openaiReq := adapter.buildOpenAIRequest(req)

	if openaiReq.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", openaiReq.Model)
	}

	if len(openaiReq.Messages) != 2 {
		t.Errorf("len(Messages) = %d, want 2", len(openaiReq.Messages))
	}

	if *openaiReq.MaxTokens != maxTokens {
		t.Errorf("MaxTokens = %d, want %d", *openaiReq.MaxTokens, maxTokens)
	}

	if *openaiReq.Temperature != temperature {
		t.Errorf("Temperature = %f, want %f", *openaiReq.Temperature, temperature)
	}

	if *openaiReq.User != user {
		t.Errorf("User = %s, want %s", *openaiReq.User, user)
	}
}

func TestConvertToUnifiedResponse(t *testing.T) {
	adapter := NewOpenAIAdapter(providers.ProviderConfig{})

	openaiResp := &OpenAIChatResponse{
		ID:      "test-123",
		Model:   "gpt-4",
		Created: 1234567890,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
		Usage: OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	req := &providers.ChatRequest{
		Model:    "gpt-4",
		Metadata: map[string]string{"session": "abc"},
	}

	latency := 200 * time.Millisecond

	resp := adapter.convertToUnifiedResponse(openaiResp, req, latency)

	if resp.ID != "test-123" {
		t.Errorf("ID = %s, want test-123", resp.ID)
	}

	if resp.Provider != "openai" {
		t.Errorf("Provider = %s, want openai", resp.Provider)
	}

	if len(resp.Choices) != 1 {
		t.Errorf("len(Choices) = %d, want 1", len(resp.Choices))
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}

	if resp.Latency != latency {
		t.Errorf("Latency = %v, want %v", resp.Latency, latency)
	}

	if resp.Metadata["session"] != "abc" {
		t.Error("Metadata not preserved")
	}
}

func BenchmarkChatCompletion(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIChatResponse{
			ID:      "test",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index:        0,
					Message:      OpenAIMessage{Role: "assistant", Content: "response"},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter := NewOpenAIAdapter(providers.ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	req := &providers.ChatRequest{
		Model:    "gpt-4",
		Messages: []providers.Message{{Role: "user", Content: "test"}},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.ChatCompletion(ctx, req)
	}
}

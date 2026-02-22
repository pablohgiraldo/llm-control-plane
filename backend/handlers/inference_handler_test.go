package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/middleware"
	"github.com/upb/llm-control-plane/backend/services"
	"go.uber.org/zap"
)

// MockInferenceService is a mock implementation of InferenceService
type MockInferenceService struct {
	mock.Mock
}

func (m *MockInferenceService) ProcessChatCompletion(ctx context.Context, req InferenceRequest) (*InferenceResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*InferenceResult), args.Error(1)
}

func TestHandleChatCompletion(t *testing.T) {
	logger := zap.NewNop()
	
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()
	
	t.Run("successful completion", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		// Mock service response
		mockResult := &InferenceResult{
			RequestID:        uuid.New().String(),
			Provider:         "openai",
			Model:            "gpt-4",
			Response:         "Hello! How can I help you?",
			FinishReason:     "stop",
			PromptTokens:     10,
			CompletionTokens: 8,
			LatencyMs:        500,
			Cost:             0.001,
			PoliciesApplied:  []uuid.UUID{uuid.New()},
		}
		
		mockService.On("ProcessChatCompletion", mock.Anything, mock.MatchedBy(func(req InferenceRequest) bool {
			return req.OrgID == orgID && req.AppID == appID && req.Model == "gpt-4"
		})).Return(mockResult, nil)
		
		// Create request
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		// Add context values
		ctx := req.Context()
		ctx = middleware.WithRequestID(ctx, uuid.New().String())
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		ctx = context.WithValue(ctx, middleware.UserIDKey, &userID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		// Call handler
		handler.HandleChatCompletion(w, req)
		
		// Assert response
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "chat.completion", data["object"])
		assert.Equal(t, "gpt-4", data["model"])
		
		choices := data["choices"].([]interface{})
		assert.Len(t, choices, 1)
		
		choice := choices[0].(map[string]interface{})
		message := choice["message"].(map[string]interface{})
		assert.Equal(t, "assistant", message["role"])
		assert.Equal(t, "Hello! How can I help you?", message["content"])
		assert.Equal(t, "stop", choice["finish_reason"])
		
		usage := data["usage"].(map[string]interface{})
		assert.Equal(t, float64(10), usage["prompt_tokens"])
		assert.Equal(t, float64(8), usage["completion_tokens"])
		assert.Equal(t, float64(18), usage["total_tokens"])
		
		mockService.AssertExpectations(t)
	})
	
	t.Run("missing tenant information", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	
	t.Run("invalid request body", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	
	t.Run("validation error - missing model", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		reqBody := ChatCompletionRequest{
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	
	t.Run("validation error - empty messages", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		reqBody := ChatCompletionRequest{
			Model:    "gpt-4",
			Messages: []ChatMessage{},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	
	t.Run("service error - rate limit exceeded", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		mockService.On("ProcessChatCompletion", mock.Anything, mock.Anything).
			Return(nil, services.ErrRateLimitExceeded)
		
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		mockService.AssertExpectations(t)
	})
	
	t.Run("service error - policy violation", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		mockService.On("ProcessChatCompletion", mock.Anything, mock.Anything).
			Return(nil, services.ErrPIIDetected)
		
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
		mockService.AssertExpectations(t)
	})
	
	t.Run("service error - provider unavailable", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		mockService.On("ProcessChatCompletion", mock.Anything, mock.Anything).
			Return(nil, services.ErrProviderUnavailable)
		
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusBadGateway, w.Code)
		mockService.AssertExpectations(t)
	})
	
	t.Run("with optional parameters", func(t *testing.T) {
		mockService := new(MockInferenceService)
		handler := NewInferenceHandler(mockService, logger)
		
		mockResult := &InferenceResult{
			RequestID:        uuid.New().String(),
			Provider:         "openai",
			Model:            "gpt-4",
			Response:         "Test response",
			FinishReason:     "stop",
			PromptTokens:     10,
			CompletionTokens: 5,
			LatencyMs:        300,
			Cost:             0.0005,
		}
		
		temp := 0.7
		maxTokens := 100
		topP := 0.9
		
		mockService.On("ProcessChatCompletion", mock.Anything, mock.MatchedBy(func(req InferenceRequest) bool {
			return req.Params["temperature"] == temp &&
				req.Params["max_tokens"] == maxTokens &&
				req.Params["top_p"] == topP
		})).Return(mockResult, nil)
		
		reqBody := ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			Temperature: &temp,
			MaxTokens:   &maxTokens,
			TopP:        &topP,
			Stop:        []string{"\n"},
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		ctx := req.Context()
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		ctx = context.WithValue(ctx, middleware.AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleChatCompletion(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectedIP     string
	}{
		{
			name: "X-Forwarded-For header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "192.168.1.1")
			},
			expectedIP: "192.168.1.1",
		},
		{
			name: "X-Real-IP header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "192.168.1.2")
			},
			expectedIP: "192.168.1.2",
		},
		{
			name: "RemoteAddr fallback",
			setupRequest: func(r *http.Request) {
				r.RemoteAddr = "192.168.1.3:8080"
			},
			expectedIP: "192.168.1.3:8080",
		},
		{
			name: "X-Forwarded-For takes precedence",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "192.168.1.1")
				r.Header.Set("X-Real-IP", "192.168.1.2")
				r.RemoteAddr = "192.168.1.3:8080"
			},
			expectedIP: "192.168.1.1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setupRequest(req)
			
			ip := getClientIP(req)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

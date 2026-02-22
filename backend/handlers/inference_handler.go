package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/middleware"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// ChatCompletionRequest represents an OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model       string                   `json:"model" validate:"required"`
	Messages    []ChatMessage            `json:"messages" validate:"required,min=1"`
	Temperature *float64                 `json:"temperature,omitempty" validate:"omitempty,gte=0,lte=2"`
	MaxTokens   *int                     `json:"max_tokens,omitempty" validate:"omitempty,gt=0"`
	TopP        *float64                 `json:"top_p,omitempty" validate:"omitempty,gte=0,lte=1"`
	Stream      bool                     `json:"stream,omitempty"`
	Stop        []string                 `json:"stop,omitempty"`
	User        string                   `json:"user,omitempty"`
	Provider    string                   `json:"provider,omitempty"` // Optional: override routing
}

// ChatMessage represents a single chat message
type ChatMessage struct {
	Role    string `json:"role" validate:"required,oneof=system user assistant"`
	Content string `json:"content" validate:"required"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatChoice           `json:"choices"`
	Usage   ChatUsage              `json:"usage"`
}

// ChatChoice represents a completion choice
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage represents token usage information
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// InferenceService defines the interface for inference operations
type InferenceService interface {
	// ProcessChatCompletion processes a chat completion request
	ProcessChatCompletion(ctx context.Context, req InferenceRequest) (*InferenceResult, error)
}

// InferenceRequest represents the service-level inference request
type InferenceRequest struct {
	OrgID     uuid.UUID
	AppID     uuid.UUID
	UserID    *uuid.UUID
	Model     string
	Provider  string // Optional: override routing
	Messages  []ChatMessage
	Params    map[string]interface{}
	IPAddress string
	UserAgent string
}

// InferenceResult represents the result of an inference request
type InferenceResult struct {
	RequestID        string
	Provider         string
	Model            string
	Response         string
	FinishReason     string
	PromptTokens     int
	CompletionTokens int
	LatencyMs        int
	Cost             float64
	PoliciesApplied  []uuid.UUID
}

// InferenceHandler handles inference-related HTTP requests
type InferenceHandler struct {
	service InferenceService
	logger  *zap.Logger
}

// NewInferenceHandler creates a new InferenceHandler
func NewInferenceHandler(service InferenceService, logger *zap.Logger) *InferenceHandler {
	return &InferenceHandler{
		service: service,
		logger:  logger,
	}
}

// HandleChatCompletion handles POST /v1/chat/completions
// Thin handler following GrantPulse pattern
func (h *InferenceHandler) HandleChatCompletion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)
	
	// Extract tenant information from context (set by middleware)
	orgID := middleware.GetOrgIDFromContext(ctx)
	appID := middleware.GetAppIDFromContext(ctx)
	userID := middleware.GetUserIDFromContext(ctx)
	
	if orgID == uuid.Nil || appID == uuid.Nil {
		h.logger.Error("missing tenant information in context")
		_ = utils.WriteUnauthorized(w, "Missing tenant information")
		return
	}
	
	// Parse request body
	var chatReq ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
		h.logger.Warn("failed to parse request body",
			zap.String("request_id", requestID),
			zap.Error(err))
		_ = utils.WriteBadRequest(w, "Invalid request body", nil)
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(&chatReq); err != nil {
		h.logger.Warn("request validation failed",
			zap.String("request_id", requestID),
			zap.Error(err))
		HandleValidationError(w, err, h.logger)
		return
	}
	
	// Build parameters map
	params := make(map[string]interface{})
	if chatReq.Temperature != nil {
		params["temperature"] = *chatReq.Temperature
	}
	if chatReq.MaxTokens != nil {
		params["max_tokens"] = *chatReq.MaxTokens
	}
	if chatReq.TopP != nil {
		params["top_p"] = *chatReq.TopP
	}
	if len(chatReq.Stop) > 0 {
		params["stop"] = chatReq.Stop
	}
	if chatReq.Stream {
		params["stream"] = true
	}
	
	// Build service request
	serviceReq := InferenceRequest{
		OrgID:     orgID,
		AppID:     appID,
		UserID:    userID,
		Model:     chatReq.Model,
		Provider:  chatReq.Provider,
		Messages:  chatReq.Messages,
		Params:    params,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
	}
	
	// Call service
	h.logger.Debug("processing chat completion",
		zap.String("request_id", requestID),
		zap.String("org_id", orgID.String()),
		zap.String("app_id", appID.String()),
		zap.String("model", chatReq.Model))
	
	result, err := h.service.ProcessChatCompletion(ctx, serviceReq)
	if err != nil {
		h.logger.Error("failed to process chat completion",
			zap.String("request_id", requestID),
			zap.Error(err))
		HandleServiceError(w, err, h.logger)
		return
	}
	
	// Build response
	response := ChatCompletionResponse{
		ID:      result.RequestID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   result.Model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: result.Response,
				},
				FinishReason: result.FinishReason,
			},
		},
		Usage: ChatUsage{
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
			TotalTokens:      result.PromptTokens + result.CompletionTokens,
		},
	}
	
	h.logger.Info("chat completion successful",
		zap.String("request_id", requestID),
		zap.String("provider", result.Provider),
		zap.String("model", result.Model),
		zap.Int("prompt_tokens", result.PromptTokens),
		zap.Int("completion_tokens", result.CompletionTokens),
		zap.Int("latency_ms", result.LatencyMs),
		zap.Float64("cost", result.Cost))
	
	// Write response
	if err := utils.WriteOK(w, response); err != nil {
		h.logger.Error("failed to write response",
			zap.String("request_id", requestID),
			zap.Error(err))
	}
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

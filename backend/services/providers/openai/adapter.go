package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/upb/llm-control-plane/backend/services/providers"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// OpenAIAdapter implements the Provider interface for OpenAI
type OpenAIAdapter struct {
	config     providers.ProviderConfig
	httpClient *http.Client
	models     map[string]*providers.ModelInfo
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(config providers.ProviderConfig) *OpenAIAdapter {
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	adapter := &OpenAIAdapter{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		models: make(map[string]*providers.ModelInfo),
	}

	// Initialize model information
	adapter.initModels()

	return adapter
}

// Name returns the provider name
func (a *OpenAIAdapter) Name() string {
	return "openai"
}

// ChatCompletion performs a chat completion request
func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	startTime := time.Now()

	// Validate model
	if err := a.ValidateModel(req.Model); err != nil {
		return nil, providers.NewProviderError(a.Name(), "INVALID_MODEL", err.Error(), 400, false, err)
	}

	// Build OpenAI request
	openaiReq := a.buildOpenAIRequest(req)

	// Marshal request
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, providers.NewProviderError(a.Name(), "MARSHAL_ERROR", "Failed to marshal request", 0, false, err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL+"/chat/completions", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, providers.NewProviderError(a.Name(), "REQUEST_ERROR", "Failed to create request", 0, false, err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.config.APIKey)
	if a.config.OrgID != "" {
		httpReq.Header.Set("OpenAI-Organization", a.config.OrgID)
	}
	for k, v := range a.config.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute request with retry logic
	var httpResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(a.config.RetryDelay * time.Duration(attempt))
		}

		httpResp, lastErr = a.httpClient.Do(httpReq)
		if lastErr == nil && httpResp.StatusCode < 500 {
			break
		}

		if httpResp != nil {
			httpResp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, providers.NewProviderError(a.Name(), "HTTP_ERROR", "HTTP request failed", 0, true, lastErr)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, providers.NewProviderError(a.Name(), "READ_ERROR", "Failed to read response", httpResp.StatusCode, false, err)
	}

	// Handle error responses
	if httpResp.StatusCode != http.StatusOK {
		return nil, a.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	// Parse response
	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, providers.NewProviderError(a.Name(), "UNMARSHAL_ERROR", "Failed to unmarshal response", httpResp.StatusCode, false, err)
	}

	// Convert to unified response
	response := a.convertToUnifiedResponse(&openaiResp, req, time.Since(startTime))

	return response, nil
}

// IsAvailable checks if the provider is currently available
func (a *OpenAIAdapter) IsAvailable(ctx context.Context) bool {
	// Simple health check - try to list models
	req, err := http.NewRequestWithContext(ctx, "GET", a.config.BaseURL+"/models", nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// EstimateCost estimates the cost for a given request
func (a *OpenAIAdapter) EstimateCost(req *providers.ChatRequest) (float64, error) {
	modelInfo, err := a.GetModelInfo(req.Model)
	if err != nil {
		return 0, err
	}

	// Rough token estimation (4 chars per token average)
	totalChars := 0
	for _, msg := range req.Messages {
		totalChars += len(msg.Content)
	}
	estimatedPromptTokens := totalChars / 4

	// Estimate completion tokens based on MaxTokens or default
	estimatedCompletionTokens := req.MaxTokens
	if estimatedCompletionTokens == 0 {
		estimatedCompletionTokens = 500 // Default estimate
	}

	promptCost := float64(estimatedPromptTokens) * modelInfo.PricingPerPromptToken
	completionCost := float64(estimatedCompletionTokens) * modelInfo.PricingPerCompletionToken

	return promptCost + completionCost, nil
}

// ValidateModel checks if a model is supported
func (a *OpenAIAdapter) ValidateModel(model string) error {
	if _, exists := a.models[model]; !exists {
		return fmt.Errorf("model %s is not supported by OpenAI provider", model)
	}
	return nil
}

// GetModelInfo returns information about a specific model
func (a *OpenAIAdapter) GetModelInfo(model string) (*providers.ModelInfo, error) {
	info, exists := a.models[model]
	if !exists {
		return nil, fmt.Errorf("model %s not found", model)
	}
	return info, nil
}

// ListModels returns all available models
func (a *OpenAIAdapter) ListModels() []string {
	models := make([]string, 0, len(a.models))
	for model := range a.models {
		models = append(models, model)
	}
	return models
}

// initModels initializes the model information map
func (a *OpenAIAdapter) initModels() {
	a.models = map[string]*providers.ModelInfo{
		"gpt-4": {
			ID:                        "gpt-4",
			Name:                      "GPT-4",
			Provider:                  "openai",
			Description:               "Most capable GPT-4 model",
			MaxTokens:                 8192,
			ContextWindow:             8192,
			PricingPerPromptToken:     0.00003,  // $0.03 per 1K tokens
			PricingPerCompletionToken: 0.00006,  // $0.06 per 1K tokens
			SupportsStreaming:         true,
			SupportsFunctions:         true,
			SupportsJSON:              true,
		},
		"gpt-4-turbo": {
			ID:                        "gpt-4-turbo",
			Name:                      "GPT-4 Turbo",
			Provider:                  "openai",
			Description:               "Latest GPT-4 Turbo with vision",
			MaxTokens:                 4096,
			ContextWindow:             128000,
			PricingPerPromptToken:     0.00001,  // $0.01 per 1K tokens
			PricingPerCompletionToken: 0.00003,  // $0.03 per 1K tokens
			SupportsStreaming:         true,
			SupportsFunctions:         true,
			SupportsVision:            true,
			SupportsJSON:              true,
		},
		"gpt-3.5-turbo": {
			ID:                        "gpt-3.5-turbo",
			Name:                      "GPT-3.5 Turbo",
			Provider:                  "openai",
			Description:               "Fast and efficient model",
			MaxTokens:                 4096,
			ContextWindow:             16385,
			PricingPerPromptToken:     0.0000005, // $0.0005 per 1K tokens
			PricingPerCompletionToken: 0.0000015, // $0.0015 per 1K tokens
			SupportsStreaming:         true,
			SupportsFunctions:         true,
			SupportsJSON:              true,
		},
		"gpt-4o": {
			ID:                        "gpt-4o",
			Name:                      "GPT-4o",
			Provider:                  "openai",
			Description:               "Optimized GPT-4 model",
			MaxTokens:                 4096,
			ContextWindow:             128000,
			PricingPerPromptToken:     0.000005,  // $0.005 per 1K tokens
			PricingPerCompletionToken: 0.000015,  // $0.015 per 1K tokens
			SupportsStreaming:         true,
			SupportsFunctions:         true,
			SupportsVision:            true,
			SupportsJSON:              true,
		},
		"gpt-4o-mini": {
			ID:                        "gpt-4o-mini",
			Name:                      "GPT-4o Mini",
			Provider:                  "openai",
			Description:               "Smaller, faster GPT-4o model",
			MaxTokens:                 16384,
			ContextWindow:             128000,
			PricingPerPromptToken:     0.00000015, // $0.00015 per 1K tokens
			PricingPerCompletionToken: 0.0000006,  // $0.0006 per 1K tokens
			SupportsStreaming:         true,
			SupportsFunctions:         true,
			SupportsJSON:              true,
		},
	}
}

// buildOpenAIRequest converts unified request to OpenAI format
func (a *OpenAIAdapter) buildOpenAIRequest(req *providers.ChatRequest) *OpenAIChatRequest {
	openaiReq := &OpenAIChatRequest{
		Model:    req.Model,
		Messages: make([]OpenAIMessage, len(req.Messages)),
		Stream:   req.Stream,
	}

	// Convert messages
	for i, msg := range req.Messages {
		openaiReq.Messages[i] = OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
		}
	}

	// Set optional parameters
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = &req.MaxTokens
	}
	if req.Temperature > 0 {
		openaiReq.Temperature = &req.Temperature
	}
	if req.TopP > 0 {
		openaiReq.TopP = &req.TopP
	}
	if len(req.Stop) > 0 {
		openaiReq.Stop = req.Stop
	}
	if req.FrequencyPenalty != 0 {
		openaiReq.FrequencyPenalty = &req.FrequencyPenalty
	}
	if req.PresencePenalty != 0 {
		openaiReq.PresencePenalty = &req.PresencePenalty
	}
	if req.User != "" {
		openaiReq.User = &req.User
	}

	return openaiReq
}

// convertToUnifiedResponse converts OpenAI response to unified format
func (a *OpenAIAdapter) convertToUnifiedResponse(openaiResp *OpenAIChatResponse, req *providers.ChatRequest, latency time.Duration) *providers.ChatResponse {
	resp := &providers.ChatResponse{
		ID:       openaiResp.ID,
		Model:    openaiResp.Model,
		Provider: a.Name(),
		Choices:  make([]providers.Choice, len(openaiResp.Choices)),
		Usage: providers.Usage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
		Latency:  latency,
		Created:  time.Unix(openaiResp.Created, 0),
		Metadata: req.Metadata,
	}

	// Convert choices
	for i, choice := range openaiResp.Choices {
		resp.Choices[i] = providers.Choice{
			Index: choice.Index,
			Message: providers.Message{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
				Name:    choice.Message.Name,
			},
			FinishReason: choice.FinishReason,
		}
	}

	return resp
}

// handleErrorResponse handles OpenAI error responses
func (a *OpenAIAdapter) handleErrorResponse(statusCode int, body []byte) error {
	var errResp OpenAIErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return providers.NewProviderError(a.Name(), "UNKNOWN_ERROR", string(body), statusCode, false, err)
	}

	retryable := statusCode >= 500 || statusCode == 429

	return providers.NewProviderError(
		a.Name(),
		errResp.Error.Type,
		errResp.Error.Message,
		statusCode,
		retryable,
		errors.New(errResp.Error.Message),
	)
}

// OpenAI-specific request/response types

type OpenAIChatRequest struct {
	Model            string          `json:"model"`
	Messages         []OpenAIMessage `json:"messages"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	User             *string         `json:"user,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type OpenAIChatResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []OpenAIChoice      `json:"choices"`
	Usage   OpenAIUsage         `json:"usage"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

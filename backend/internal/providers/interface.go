package providers

import (
	"context"
	"time"
)

// Provider represents an LLM provider (OpenAI, Anthropic, Azure, etc.).
type Provider interface {
	Name() string
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	IsAvailable(ctx context.Context) bool
	CalculateCost(req *ChatRequest, resp *ChatResponse) (float64, error)
}

// ChatRequest represents a unified chat completion request.
type ChatRequest struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Stream      bool
	Metadata    map[string]string
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ChatResponse represents a unified chat completion response.
type ChatResponse struct {
	ID       string
	Model    string
	Choices  []Choice
	Usage    Usage
	Provider string
	Latency  time.Duration
}

// Choice represents a completion choice.
type Choice struct {
	Index        int
	Message      Message
	FinishReason string
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// TODO: Implement provider adapters for OpenAI, Anthropic, and Azure

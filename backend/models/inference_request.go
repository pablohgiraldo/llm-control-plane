package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// InferenceStatus represents the status of an inference request
type InferenceStatus string

const (
	InferenceStatusPending   InferenceStatus = "pending"
	InferenceStatusProcessing InferenceStatus = "processing"
	InferenceStatusCompleted  InferenceStatus = "completed"
	InferenceStatusFailed     InferenceStatus = "failed"
	InferenceStatusRejected   InferenceStatus = "rejected" // Rejected by policy
)

// InferenceRequest represents a request to an LLM provider
type InferenceRequest struct {
	ID               uuid.UUID       `json:"id" db:"id"`
	OrgID            uuid.UUID       `json:"org_id" db:"org_id"`
	AppID            uuid.UUID       `json:"app_id" db:"app_id"`
	UserID           *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	RequestID        string          `json:"request_id" db:"request_id"`       // External request ID
	Status           InferenceStatus `json:"status" db:"status"`
	
	// Provider details
	Provider         string          `json:"provider" db:"provider"`           // openai, anthropic, bedrock
	Model            string          `json:"model" db:"model"`
	
	// Request content
	Prompt           string          `json:"prompt" db:"prompt"`
	Messages         json.RawMessage `json:"messages,omitempty" db:"messages"` // For chat-based models
	Parameters       json.RawMessage `json:"parameters" db:"parameters"`       // Model parameters (temperature, etc.)
	
	// Response content
	Response         *string         `json:"response,omitempty" db:"response"`
	FinishReason     *string         `json:"finish_reason,omitempty" db:"finish_reason"`
	
	// Metrics
	PromptTokens     int             `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens" db:"total_tokens"`
	LatencyMs        int             `json:"latency_ms" db:"latency_ms"`
	Cost             float64         `json:"cost" db:"cost"`
	
	// Policy enforcement
	PoliciesApplied  json.RawMessage `json:"policies_applied" db:"policies_applied"` // List of policy IDs
	PolicyViolations json.RawMessage `json:"policy_violations,omitempty" db:"policy_violations"`
	
	// Timestamps
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	StartedAt        *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	
	// Error handling
	ErrorMessage     *string         `json:"error_message,omitempty" db:"error_message"`
	ErrorCode        *string         `json:"error_code,omitempty" db:"error_code"`
	
	// Request metadata
	IPAddress        string          `json:"ip_address" db:"ip_address"`
	UserAgent        string          `json:"user_agent" db:"user_agent"`
}

// TableName returns the table name for the InferenceRequest model
func (InferenceRequest) TableName() string {
	return "inference_requests"
}

// NewInferenceRequest creates a new InferenceRequest instance
func NewInferenceRequest(orgID, appID uuid.UUID, provider, model, prompt string) *InferenceRequest {
	return &InferenceRequest{
		ID:        uuid.New(),
		OrgID:     orgID,
		AppID:     appID,
		RequestID: uuid.New().String(),
		Status:    InferenceStatusPending,
		Provider:  provider,
		Model:     model,
		Prompt:    prompt,
		CreatedAt: time.Now(),
	}
}

// MarkAsProcessing marks the request as processing
func (ir *InferenceRequest) MarkAsProcessing() {
	ir.Status = InferenceStatusProcessing
	now := time.Now()
	ir.StartedAt = &now
}

// MarkAsCompleted marks the request as completed
func (ir *InferenceRequest) MarkAsCompleted(response, finishReason string, promptTokens, completionTokens, latencyMs int, cost float64) {
	ir.Status = InferenceStatusCompleted
	ir.Response = &response
	ir.FinishReason = &finishReason
	ir.PromptTokens = promptTokens
	ir.CompletionTokens = completionTokens
	ir.TotalTokens = promptTokens + completionTokens
	ir.LatencyMs = latencyMs
	ir.Cost = cost
	now := time.Now()
	ir.CompletedAt = &now
}

// MarkAsFailed marks the request as failed
func (ir *InferenceRequest) MarkAsFailed(errorCode, errorMessage string) {
	ir.Status = InferenceStatusFailed
	ir.ErrorCode = &errorCode
	ir.ErrorMessage = &errorMessage
	now := time.Now()
	ir.CompletedAt = &now
}

// MarkAsRejected marks the request as rejected by policy
func (ir *InferenceRequest) MarkAsRejected(reason string, violations interface{}) {
	ir.Status = InferenceStatusRejected
	ir.ErrorMessage = &reason
	if data, err := json.Marshal(violations); err == nil {
		ir.PolicyViolations = data
	}
	now := time.Now()
	ir.CompletedAt = &now
}

// SetPoliciesApplied sets the list of applied policy IDs
func (ir *InferenceRequest) SetPoliciesApplied(policyIDs []uuid.UUID) {
	if data, err := json.Marshal(policyIDs); err == nil {
		ir.PoliciesApplied = data
	}
}

// SetUser sets the user ID
func (ir *InferenceRequest) SetUser(userID uuid.UUID) {
	ir.UserID = &userID
}

// SetRequestMetadata sets request metadata
func (ir *InferenceRequest) SetRequestMetadata(ipAddress, userAgent string) {
	ir.IPAddress = ipAddress
	ir.UserAgent = userAgent
}

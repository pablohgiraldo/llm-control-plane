package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	AuditActionInferenceRequest  AuditAction = "inference_request"
	AuditActionInferenceResponse AuditAction = "inference_response"
	AuditActionPolicyViolation   AuditAction = "policy_violation"
	AuditActionPolicyCreated     AuditAction = "policy_created"
	AuditActionPolicyUpdated     AuditAction = "policy_updated"
	AuditActionPolicyDeleted     AuditAction = "policy_deleted"
	AuditActionUserCreated       AuditAction = "user_created"
	AuditActionUserUpdated       AuditAction = "user_updated"
	AuditActionAppCreated        AuditAction = "app_created"
	AuditActionAppUpdated        AuditAction = "app_updated"
)

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	OrgID          uuid.UUID       `json:"org_id" db:"org_id"`
	AppID          *uuid.UUID      `json:"app_id,omitempty" db:"app_id"`
	UserID         *uuid.UUID      `json:"user_id,omitempty" db:"user_id"`
	Action         AuditAction     `json:"action" db:"action"`
	ResourceType   string          `json:"resource_type" db:"resource_type"` // policy, user, app, etc.
	ResourceID     *uuid.UUID      `json:"resource_id,omitempty" db:"resource_id"`
	Details        json.RawMessage `json:"details" db:"details"` // JSONB for flexible metadata
	IPAddress      string          `json:"ip_address" db:"ip_address"`
	UserAgent      string          `json:"user_agent" db:"user_agent"`
	RequestID      string          `json:"request_id" db:"request_id"`
	Timestamp      time.Time       `json:"timestamp" db:"timestamp"`
	
	// LLM-specific fields
	Model          *string         `json:"model,omitempty" db:"model"`
	Provider       *string         `json:"provider,omitempty" db:"provider"`
	TokensUsed     *int            `json:"tokens_used,omitempty" db:"tokens_used"`
	Cost           *float64        `json:"cost,omitempty" db:"cost"`
	LatencyMs      *int            `json:"latency_ms,omitempty" db:"latency_ms"`
	StatusCode     *int            `json:"status_code,omitempty" db:"status_code"`
	ErrorMessage   *string         `json:"error_message,omitempty" db:"error_message"`
}

// TableName returns the table name for the AuditLog model
func (AuditLog) TableName() string {
	return "audit_logs"
}

// NewAuditLog creates a new AuditLog instance
func NewAuditLog(orgID uuid.UUID, action AuditAction, resourceType string) *AuditLog {
	return &AuditLog{
		ID:           uuid.New(),
		OrgID:        orgID,
		Action:       action,
		ResourceType: resourceType,
		Timestamp:    time.Now(),
	}
}

// WithApp sets the application ID
func (a *AuditLog) WithApp(appID uuid.UUID) *AuditLog {
	a.AppID = &appID
	return a
}

// WithUser sets the user ID
func (a *AuditLog) WithUser(userID uuid.UUID) *AuditLog {
	a.UserID = &userID
	return a
}

// WithResource sets the resource ID
func (a *AuditLog) WithResource(resourceID uuid.UUID) *AuditLog {
	a.ResourceID = &resourceID
	return a
}

// WithDetails sets the details
func (a *AuditLog) WithDetails(details interface{}) *AuditLog {
	if data, err := json.Marshal(details); err == nil {
		a.Details = data
	}
	return a
}

// WithRequest sets request metadata
func (a *AuditLog) WithRequest(requestID, ipAddress, userAgent string) *AuditLog {
	a.RequestID = requestID
	a.IPAddress = ipAddress
	a.UserAgent = userAgent
	return a
}

// WithLLMMetrics sets LLM-specific metrics
func (a *AuditLog) WithLLMMetrics(model, provider string, tokensUsed, latencyMs int, cost float64) *AuditLog {
	a.Model = &model
	a.Provider = &provider
	a.TokensUsed = &tokensUsed
	a.LatencyMs = &latencyMs
	a.Cost = &cost
	return a
}

// WithError sets error information
func (a *AuditLog) WithError(statusCode int, errorMessage string) *AuditLog {
	a.StatusCode = &statusCode
	a.ErrorMessage = &errorMessage
	return a
}

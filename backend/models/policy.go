package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PolicyType represents different types of policies
type PolicyType string

const (
	PolicyTypeRateLimit   PolicyType = "rate_limit"
	PolicyTypeBudget      PolicyType = "budget"
	PolicyTypeRouting     PolicyType = "routing"
	PolicyTypePII         PolicyType = "pii_detection"
	PolicyTypeInjection   PolicyType = "injection_guard"
	PolicyTypeRAG         PolicyType = "rag"
	PolicyTypeRetry       PolicyType = "retry"
	PolicyTypeFallback    PolicyType = "fallback"
	PolicyTypeLoadBalance PolicyType = "load_balance"
)

// Policy represents a policy configuration for controlling LLM behavior
type Policy struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	OrgID      uuid.UUID       `json:"org_id" db:"org_id"`
	AppID      *uuid.UUID      `json:"app_id,omitempty" db:"app_id"`   // Null if org-wide
	UserID     *uuid.UUID      `json:"user_id,omitempty" db:"user_id"` // Null if not user-specific
	PolicyType PolicyType      `json:"policy_type" db:"policy_type"`
	Config     json.RawMessage `json:"config" db:"config"` // JSONB configuration
	Priority   int             `json:"priority" db:"priority"`
	Enabled    bool            `json:"enabled" db:"enabled"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the Policy model
func (Policy) TableName() string {
	return "policies"
}

// NewPolicy creates a new Policy instance
func NewPolicy(orgID uuid.UUID, policyType PolicyType, config json.RawMessage, priority int) *Policy {
	now := time.Now()
	return &Policy{
		ID:         uuid.New(),
		OrgID:      orgID,
		PolicyType: policyType,
		Config:     config,
		Priority:   priority,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// RateLimitConfig represents rate limiting policy configuration
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	RequestsPerHour   int `json:"requests_per_hour"`
	RequestsPerDay    int `json:"requests_per_day"`
	TokensPerMinute   int `json:"tokens_per_minute"`
	TokensPerHour     int `json:"tokens_per_hour"`
	TokensPerDay      int `json:"tokens_per_day"`
}

// BudgetConfig represents budget policy configuration
type BudgetConfig struct {
	MaxCostPerRequest float64 `json:"max_cost_per_request"`
	MaxDailyCost      float64 `json:"max_daily_cost"`
	MaxMonthlyCost    float64 `json:"max_monthly_cost"`
	Currency          string  `json:"currency"`
}

// RoutingConfig represents routing policy configuration
type RoutingConfig struct {
	PrimaryProvider  string   `json:"primary_provider"`
	FallbackProviders []string `json:"fallback_providers"`
	Strategy         string   `json:"strategy"` // least_latency, cost_optimized, etc.
}

// PIIConfig represents PII detection policy configuration
type PIIConfig struct {
	Enabled         bool     `json:"enabled"`
	RedactTypes     []string `json:"redact_types"`     // email, phone, ssn, etc.
	BlockOnDetection bool     `json:"block_on_detection"`
}

// InjectionGuardConfig represents injection guard policy configuration
type InjectionGuardConfig struct {
	Enabled          bool     `json:"enabled"`
	DetectionMethods []string `json:"detection_methods"` // heuristic, llm_based
	BlockOnDetection bool     `json:"block_on_detection"`
}

// RAGConfig represents RAG policy configuration
type RAGConfig struct {
	Enabled        bool   `json:"enabled"`
	IndexName      string `json:"index_name"`
	TopK           int    `json:"top_k"`
	ScoreThreshold float64 `json:"score_threshold"`
}

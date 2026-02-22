package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Organization tests
func TestNewOrganization(t *testing.T) {
	name := "Test Organization"
	slug := "test-org"

	org := NewOrganization(name, slug)

	assert.NotEqual(t, uuid.Nil, org.ID)
	assert.Equal(t, name, org.Name)
	assert.Equal(t, slug, org.Slug)
	assert.False(t, org.CreatedAt.IsZero())
	assert.False(t, org.UpdatedAt.IsZero())
	assert.Equal(t, org.CreatedAt, org.UpdatedAt)
}

func TestOrganization_TableName(t *testing.T) {
	org := Organization{}
	assert.Equal(t, "organizations", org.TableName())
}

// Application tests
func TestNewApplication(t *testing.T) {
	orgID := uuid.New()
	name := "Test App"
	apiKeyHash := "hashed_key"

	app := NewApplication(orgID, name, apiKeyHash)

	assert.NotEqual(t, uuid.Nil, app.ID)
	assert.Equal(t, orgID, app.OrgID)
	assert.Equal(t, name, app.Name)
	assert.Equal(t, apiKeyHash, app.APIKeyHash)
	assert.False(t, app.CreatedAt.IsZero())
	assert.False(t, app.UpdatedAt.IsZero())
}

func TestApplication_TableName(t *testing.T) {
	app := Application{}
	assert.Equal(t, "applications", app.TableName())
}

func TestApplication_JSONMarshaling(t *testing.T) {
	app := Application{
		ID:         uuid.New(),
		OrgID:      uuid.New(),
		Name:       "Test",
		APIKeyHash: "secret",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, err := json.Marshal(app)
	require.NoError(t, err)

	// Verify API key hash is not in JSON
	assert.NotContains(t, string(data), "secret")
	assert.NotContains(t, string(data), "api_key_hash")
}

// User tests
func TestNewUser(t *testing.T) {
	email := "test@example.com"
	cognitoSub := "cognito-sub-123"
	orgID := uuid.New()
	role := RoleAdmin

	user := NewUser(email, cognitoSub, orgID, role)

	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, cognitoSub, user.CognitoSub)
	assert.Equal(t, orgID, user.OrgID)
	assert.Equal(t, role, user.Role)
	assert.False(t, user.CreatedAt.IsZero())
}

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"admin", RoleAdmin, true},
		{"member", RoleMember, false},
		{"viewer", RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Role: tt.role}
			assert.Equal(t, tt.want, user.IsAdmin())
		})
	}
}

func TestUser_CanManagePolicies(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"admin", RoleAdmin, true},
		{"member", RoleMember, true},
		{"viewer", RoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Role: tt.role}
			assert.Equal(t, tt.want, user.CanManagePolicies())
		})
	}
}

func TestUser_TableName(t *testing.T) {
	user := User{}
	assert.Equal(t, "users", user.TableName())
}

// Policy tests
func TestNewPolicy(t *testing.T) {
	orgID := uuid.New()
	policyType := PolicyTypeRateLimit
	config := json.RawMessage(`{"requests_per_minute": 100}`)
	priority := 10

	policy := NewPolicy(orgID, policyType, config, priority)

	assert.NotEqual(t, uuid.Nil, policy.ID)
	assert.Equal(t, orgID, policy.OrgID)
	assert.Equal(t, policyType, policy.PolicyType)
	assert.Equal(t, config, policy.Config)
	assert.Equal(t, priority, policy.Priority)
	assert.True(t, policy.Enabled)
	assert.False(t, policy.CreatedAt.IsZero())
}

func TestPolicy_TableName(t *testing.T) {
	policy := Policy{}
	assert.Equal(t, "policies", policy.TableName())
}

func TestPolicyConfigTypes(t *testing.T) {
	t.Run("RateLimitConfig", func(t *testing.T) {
		config := RateLimitConfig{
			RequestsPerMinute: 100,
			RequestsPerHour:   1000,
			TokensPerMinute:   50000,
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var decoded RateLimitConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, config, decoded)
	})

	t.Run("BudgetConfig", func(t *testing.T) {
		config := BudgetConfig{
			MaxCostPerRequest: 0.10,
			MaxDailyCost:      100.0,
			MaxMonthlyCost:    3000.0,
			Currency:          "USD",
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var decoded BudgetConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, config, decoded)
	})

	t.Run("RoutingConfig", func(t *testing.T) {
		config := RoutingConfig{
			PrimaryProvider:  "openai",
			FallbackProviders: []string{"anthropic", "bedrock"},
			Strategy:         "least_latency",
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var decoded RoutingConfig
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, config, decoded)
	})
}

// AuditLog tests
func TestNewAuditLog(t *testing.T) {
	orgID := uuid.New()
	action := AuditActionPolicyCreated
	resourceType := "policy"

	log := NewAuditLog(orgID, action, resourceType)

	assert.NotEqual(t, uuid.Nil, log.ID)
	assert.Equal(t, orgID, log.OrgID)
	assert.Equal(t, action, log.Action)
	assert.Equal(t, resourceType, log.ResourceType)
	assert.False(t, log.Timestamp.IsZero())
}

func TestAuditLog_BuilderMethods(t *testing.T) {
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()
	resourceID := uuid.New()

	log := NewAuditLog(orgID, AuditActionInferenceRequest, "inference").
		WithApp(appID).
		WithUser(userID).
		WithResource(resourceID).
		WithRequest("req-123", "192.168.1.1", "Mozilla/5.0").
		WithLLMMetrics("gpt-4", "openai", 1000, 500, 0.02).
		WithDetails(map[string]interface{}{"key": "value"})

	assert.Equal(t, appID, *log.AppID)
	assert.Equal(t, userID, *log.UserID)
	assert.Equal(t, resourceID, *log.ResourceID)
	assert.Equal(t, "req-123", log.RequestID)
	assert.Equal(t, "192.168.1.1", log.IPAddress)
	assert.Equal(t, "Mozilla/5.0", log.UserAgent)
	assert.Equal(t, "gpt-4", *log.Model)
	assert.Equal(t, "openai", *log.Provider)
	assert.Equal(t, 1000, *log.TokensUsed)
	assert.Equal(t, 500, *log.LatencyMs)
	assert.Equal(t, 0.02, *log.Cost)
	assert.NotNil(t, log.Details)
}

func TestAuditLog_TableName(t *testing.T) {
	log := AuditLog{}
	assert.Equal(t, "audit_logs", log.TableName())
}

// InferenceRequest tests
func TestNewInferenceRequest(t *testing.T) {
	orgID := uuid.New()
	appID := uuid.New()
	provider := "openai"
	model := "gpt-4"
	prompt := "Hello, world!"

	req := NewInferenceRequest(orgID, appID, provider, model, prompt)

	assert.NotEqual(t, uuid.Nil, req.ID)
	assert.Equal(t, orgID, req.OrgID)
	assert.Equal(t, appID, req.AppID)
	assert.Equal(t, provider, req.Provider)
	assert.Equal(t, model, req.Model)
	assert.Equal(t, prompt, req.Prompt)
	assert.Equal(t, InferenceStatusPending, req.Status)
	assert.NotEmpty(t, req.RequestID)
	assert.False(t, req.CreatedAt.IsZero())
}

func TestInferenceRequest_MarkAsProcessing(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")

	req.MarkAsProcessing()

	assert.Equal(t, InferenceStatusProcessing, req.Status)
	assert.NotNil(t, req.StartedAt)
	assert.False(t, req.StartedAt.IsZero())
}

func TestInferenceRequest_MarkAsCompleted(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")

	response := "Test response"
	finishReason := "stop"
	promptTokens := 10
	completionTokens := 20
	latencyMs := 500
	cost := 0.001

	req.MarkAsCompleted(response, finishReason, promptTokens, completionTokens, latencyMs, cost)

	assert.Equal(t, InferenceStatusCompleted, req.Status)
	assert.Equal(t, response, *req.Response)
	assert.Equal(t, finishReason, *req.FinishReason)
	assert.Equal(t, promptTokens, req.PromptTokens)
	assert.Equal(t, completionTokens, req.CompletionTokens)
	assert.Equal(t, promptTokens+completionTokens, req.TotalTokens)
	assert.Equal(t, latencyMs, req.LatencyMs)
	assert.Equal(t, cost, req.Cost)
	assert.NotNil(t, req.CompletedAt)
}

func TestInferenceRequest_MarkAsFailed(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")

	errorCode := "timeout"
	errorMessage := "Request timed out"

	req.MarkAsFailed(errorCode, errorMessage)

	assert.Equal(t, InferenceStatusFailed, req.Status)
	assert.Equal(t, errorCode, *req.ErrorCode)
	assert.Equal(t, errorMessage, *req.ErrorMessage)
	assert.NotNil(t, req.CompletedAt)
}

func TestInferenceRequest_MarkAsRejected(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")

	reason := "Policy violation"
	violations := map[string]interface{}{
		"policy_type": "rate_limit",
		"limit":       100,
	}

	req.MarkAsRejected(reason, violations)

	assert.Equal(t, InferenceStatusRejected, req.Status)
	assert.Equal(t, reason, *req.ErrorMessage)
	assert.NotNil(t, req.PolicyViolations)
	assert.NotNil(t, req.CompletedAt)

	var decoded map[string]interface{}
	err := json.Unmarshal(req.PolicyViolations, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "rate_limit", decoded["policy_type"])
}

func TestInferenceRequest_SetPoliciesApplied(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")

	policyIDs := []uuid.UUID{uuid.New(), uuid.New()}
	req.SetPoliciesApplied(policyIDs)

	assert.NotNil(t, req.PoliciesApplied)

	var decoded []uuid.UUID
	err := json.Unmarshal(req.PoliciesApplied, &decoded)
	require.NoError(t, err)
	assert.Equal(t, policyIDs, decoded)
}

func TestInferenceRequest_TableName(t *testing.T) {
	req := InferenceRequest{}
	assert.Equal(t, "inference_requests", req.TableName())
}

func TestInferenceRequest_SetUser(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")
	userID := uuid.New()

	req.SetUser(userID)

	assert.NotNil(t, req.UserID)
	assert.Equal(t, userID, *req.UserID)
}

func TestInferenceRequest_SetRequestMetadata(t *testing.T) {
	req := NewInferenceRequest(uuid.New(), uuid.New(), "openai", "gpt-4", "test")
	ipAddress := "192.168.1.1"
	userAgent := "Mozilla/5.0"

	req.SetRequestMetadata(ipAddress, userAgent)

	assert.Equal(t, ipAddress, req.IPAddress)
	assert.Equal(t, userAgent, req.UserAgent)
}

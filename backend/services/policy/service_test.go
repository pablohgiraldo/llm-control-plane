package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// MockPolicyRepository is a mock implementation of PolicyRepository
type MockPolicyRepository struct {
	mock.Mock
}

func (m *MockPolicyRepository) Create(ctx context.Context, policy *models.Policy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockPolicyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Policy, error) {
	args := m.Called(ctx, id)
	if policy := args.Get(0); policy != nil {
		return policy.(*models.Policy), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPolicyRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, orgID)
	if policies := args.Get(0); policies != nil {
		return policies.([]*models.Policy), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPolicyRepository) GetByAppID(ctx context.Context, appID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, appID)
	if policies := args.Get(0); policies != nil {
		return policies.([]*models.Policy), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPolicyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, userID)
	if policies := args.Get(0); policies != nil {
		return policies.([]*models.Policy), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPolicyRepository) GetApplicablePolicies(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, orgID, appID, userID)
	if policies := args.Get(0); policies != nil {
		return policies.([]*models.Policy), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPolicyRepository) Update(ctx context.Context, policy *models.Policy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockPolicyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPolicyRepository) WithTx(tx repositories.Transaction) repositories.PolicyRepository {
	args := m.Called(tx)
	return args.Get(0).(repositories.PolicyRepository)
}

func TestPolicyService_Evaluate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	// Create test policies
	rateLimitConfig := models.RateLimitConfig{
		RequestsPerMinute: 100,
		RequestsPerHour:   1000,
		TokensPerMinute:   10000,
	}
	rateLimitJSON, _ := json.Marshal(rateLimitConfig)

	budgetConfig := models.BudgetConfig{
		MaxDailyCost:   100.0,
		MaxMonthlyCost: 3000.0,
		Currency:       "USD",
	}
	budgetJSON, _ := json.Marshal(budgetConfig)

	orgPolicies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON,
			Priority:   10,
			Enabled:    true,
		},
	}

	appPolicies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			AppID:      &appID,
			PolicyType: models.PolicyTypeBudget,
			Config:     budgetJSON,
			Priority:   20,
			Enabled:    true,
		},
	}

	userPolicies := []*models.Policy{}

	mockRepo.On("GetByOrgID", ctx, orgID).Return(orgPolicies, nil)
	mockRepo.On("GetByAppID", ctx, appID).Return(appPolicies, nil)
	mockRepo.On("GetByUserID", ctx, userID).Return(userPolicies, nil)

	req := EvaluationRequest{
		OrgID:    orgID,
		AppID:    appID,
		UserID:   &userID,
		Provider: "openai",
		Model:    "gpt-4",
		Prompt:   "test prompt",
	}

	result, err := service.Evaluate(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Allowed)
	assert.Equal(t, 2, len(result.AppliedPolicies))
	assert.NotNil(t, result.RateLimitConfig)
	assert.NotNil(t, result.BudgetConfig)
	assert.Equal(t, 100, result.RateLimitConfig.RequestsPerMinute)
	assert.Equal(t, 100.0, result.BudgetConfig.MaxDailyCost)

	mockRepo.AssertExpectations(t)
}

func TestPolicyService_MergePolicies_Priority(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	rateLimitConfig1 := models.RateLimitConfig{RequestsPerMinute: 100}
	rateLimitJSON1, _ := json.Marshal(rateLimitConfig1)

	rateLimitConfig2 := models.RateLimitConfig{RequestsPerMinute: 200}
	rateLimitJSON2, _ := json.Marshal(rateLimitConfig2)

	rateLimitConfig3 := models.RateLimitConfig{RequestsPerMinute: 300}
	rateLimitJSON3, _ := json.Marshal(rateLimitConfig3)

	policies := []*models.Policy{
		// Org-level policy (priority 10)
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON1,
			Priority:   10,
			Enabled:    true,
		},
		// App-level policy (priority 20)
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			AppID:      &appID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON2,
			Priority:   20,
			Enabled:    true,
		},
		// User-level policy (priority 30)
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			AppID:      &appID,
			UserID:     &userID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON3,
			Priority:   30,
			Enabled:    true,
		},
	}

	merged := service.mergePolicies(policies)

	// Should select the user-level policy (highest level)
	assert.Equal(t, 1, len(merged))
	assert.NotNil(t, merged[0].UserID)
	assert.Equal(t, *merged[0].UserID, userID)
	
	var config models.RateLimitConfig
	json.Unmarshal(merged[0].Config, &config)
	assert.Equal(t, 300, config.RequestsPerMinute)
}

func TestPolicyService_MergePolicies_SameLevel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()

	rateLimitConfig1 := models.RateLimitConfig{RequestsPerMinute: 100}
	rateLimitJSON1, _ := json.Marshal(rateLimitConfig1)

	rateLimitConfig2 := models.RateLimitConfig{RequestsPerMinute: 200}
	rateLimitJSON2, _ := json.Marshal(rateLimitConfig2)

	policies := []*models.Policy{
		// Org-level policy (priority 10)
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON1,
			Priority:   10,
			Enabled:    true,
		},
		// Org-level policy (priority 20) - should win
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON2,
			Priority:   20,
			Enabled:    true,
		},
	}

	merged := service.mergePolicies(policies)

	// Should select the higher priority policy
	assert.Equal(t, 1, len(merged))
	assert.Nil(t, merged[0].AppID)
	assert.Nil(t, merged[0].UserID)
	
	var config models.RateLimitConfig
	json.Unmarshal(merged[0].Config, &config)
	assert.Equal(t, 200, config.RequestsPerMinute)
}

func TestPolicyService_MergePolicies_MultipleTypes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()

	rateLimitConfig := models.RateLimitConfig{RequestsPerMinute: 100}
	rateLimitJSON, _ := json.Marshal(rateLimitConfig)

	budgetConfig := models.BudgetConfig{MaxDailyCost: 100.0}
	budgetJSON, _ := json.Marshal(budgetConfig)

	policies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON,
			Priority:   10,
			Enabled:    true,
		},
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeBudget,
			Config:     budgetJSON,
			Priority:   10,
			Enabled:    true,
		},
	}

	merged := service.mergePolicies(policies)

	// Should have both types
	assert.Equal(t, 2, len(merged))

	typeCount := make(map[models.PolicyType]int)
	for _, p := range merged {
		typeCount[p.PolicyType]++
	}
	assert.Equal(t, 1, typeCount[models.PolicyTypeRateLimit])
	assert.Equal(t, 1, typeCount[models.PolicyTypeBudget])
}

func TestPolicyService_MergePolicies_DisabledPolicies(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()

	rateLimitConfig := models.RateLimitConfig{RequestsPerMinute: 100}
	rateLimitJSON, _ := json.Marshal(rateLimitConfig)

	policies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     rateLimitJSON,
			Priority:   10,
			Enabled:    false, // Disabled
		},
	}

	merged := service.mergePolicies(policies)

	// Should not include disabled policies
	assert.Equal(t, 0, len(merged))
}

func TestPolicyService_CacheInvalidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	ctx := context.Background()
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	orgPolicies := []*models.Policy{}
	appPolicies := []*models.Policy{}
	userPolicies := []*models.Policy{}

	mockRepo.On("GetByOrgID", ctx, orgID).Return(orgPolicies, nil)
	mockRepo.On("GetByAppID", ctx, appID).Return(appPolicies, nil)
	mockRepo.On("GetByUserID", ctx, userID).Return(userPolicies, nil)

	req := EvaluationRequest{
		OrgID:  orgID,
		AppID:  appID,
		UserID: &userID,
	}

	// First call - cache miss
	result1, err := service.Evaluate(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result1)

	// Second call - cache hit (should not call repo again)
	result2, err := service.Evaluate(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result2)

	// Verify repo was only called once
	mockRepo.AssertNumberOfCalls(t, "GetByOrgID", 1)
	mockRepo.AssertNumberOfCalls(t, "GetByAppID", 1)
	mockRepo.AssertNumberOfCalls(t, "GetByUserID", 1)

	// Invalidate cache
	service.InvalidateCache(orgID, appID, &userID)

	// Third call - cache miss again (should call repo)
	result3, err := service.Evaluate(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result3)

	// Should have made 6 total calls (2 evaluations * 3 repo calls each)
	mockRepo.AssertNumberOfCalls(t, "GetByOrgID", 2)
	mockRepo.AssertNumberOfCalls(t, "GetByAppID", 2)
	mockRepo.AssertNumberOfCalls(t, "GetByUserID", 2)
}

func TestPolicyService_GetPolicyLevel(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	orgPolicy := &models.Policy{
		OrgID: orgID,
	}
	assert.Equal(t, 1, service.getPolicyLevel(orgPolicy))

	appPolicy := &models.Policy{
		OrgID: orgID,
		AppID: &appID,
	}
	assert.Equal(t, 2, service.getPolicyLevel(appPolicy))

	userPolicy := &models.Policy{
		OrgID:  orgID,
		AppID:  &appID,
		UserID: &userID,
	}
	assert.Equal(t, 3, service.getPolicyLevel(userPolicy))
}

func TestPolicyService_InvalidateCacheForOrg(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	orgID := uuid.New()
	appID1 := uuid.New()
	appID2 := uuid.New()

	// Add cache entries for different apps
	cache.SetPolicies(CacheKey{OrgID: orgID, AppID: appID1}, []*models.Policy{})
	cache.SetPolicies(CacheKey{OrgID: orgID, AppID: appID2}, []*models.Policy{})

	assert.Equal(t, 2, cache.Stats().Size)

	// Invalidate entire org
	service.InvalidateCacheForOrg(orgID)

	// All entries should be removed
	assert.Equal(t, 0, cache.Stats().Size)
}

func TestPolicyService_GetCacheStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cache := NewPolicyCache(10, 5*time.Minute)
	mockRepo := new(MockPolicyRepository)
	service := NewPolicyService(mockRepo, cache, logger)

	stats := service.GetCacheStats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 10, stats.MaxSize)
}

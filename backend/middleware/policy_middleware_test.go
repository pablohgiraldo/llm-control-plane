package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/services/policy"
	"github.com/upb/llm-control-plane/backend/services/ratelimit"
	"go.uber.org/zap"
)

// MockPolicyService is a mock implementation of PolicyService
type MockPolicyService struct {
	mock.Mock
}

func (m *MockPolicyService) Evaluate(ctx context.Context, req policy.EvaluationRequest) (*policy.EvaluationResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*policy.EvaluationResult), args.Error(1)
}

// MockRateLimitService is a mock implementation of RateLimitService
type MockRateLimitService struct {
	mock.Mock
}

func (m *MockRateLimitService) CheckLimit(ctx context.Context, req ratelimit.RateLimitRequest) (*ratelimit.RateLimitResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ratelimit.RateLimitResult), args.Error(1)
}

func (m *MockRateLimitService) RecordRequest(ctx context.Context, req ratelimit.RateLimitRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func TestEnforcePolicy(t *testing.T) {
	logger := zap.NewNop()
	
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()
	
	t.Run("successful policy enforcement - no policies", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		// Mock policy evaluation - allowed with no policies
		evalResult := &policy.EvaluationResult{
			Allowed:         true,
			AppliedPolicies: []*models.Policy{},
			Violations:      []policy.PolicyViolation{},
		}
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.MatchedBy(func(req policy.EvaluationRequest) bool {
			return req.OrgID == orgID && req.AppID == appID
		})).Return(evalResult, nil)
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify policy result is in context
			ctx := r.Context()
			result := GetPolicyResultFromContext(ctx)
			assert.NotNil(t, result)
			assert.True(t, result.Allowed)
			
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		ctx = context.WithValue(ctx, UserIDKey, &userID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		mockPolicyService.AssertExpectations(t)
	})
	
	t.Run("successful with rate limit policy", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		// Mock policy evaluation with rate limit
		rateLimitConfig := &models.RateLimitConfig{
			RequestsPerMinute: 100,
		}
		evalResult := &policy.EvaluationResult{
			Allowed:         true,
			AppliedPolicies: []*models.Policy{},
			Violations:      []policy.PolicyViolation{},
			RateLimitConfig: rateLimitConfig,
		}
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.Anything).Return(evalResult, nil)
		
		// Mock rate limit check - allowed
		rateLimitResult := &ratelimit.RateLimitResult{
			Allowed:           true,
			RequestsRemaining: 50,
			ResetAt:           time.Now().Add(30 * time.Second),
		}
		mockRateLimitService.On("CheckLimit", mock.Anything, mock.MatchedBy(func(req ratelimit.RateLimitRequest) bool {
			return req.OrgID == orgID && req.AppID == appID && req.Config == rateLimitConfig
		})).Return(rateLimitResult, nil)
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
		
		mockPolicyService.AssertExpectations(t)
		mockRateLimitService.AssertExpectations(t)
	})
	
	t.Run("missing tenant information", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	
	t.Run("policy evaluation fails", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.Anything).
			Return(nil, errors.New("database error"))
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockPolicyService.AssertExpectations(t)
	})
	
	t.Run("request blocked by policy", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		// Mock policy evaluation - not allowed
		evalResult := &policy.EvaluationResult{
			Allowed: false,
			Violations: []policy.PolicyViolation{
				{
					PolicyID:   uuid.New(),
					PolicyType: models.PolicyTypePII,
					Reason:     "PII detected in prompt",
				},
			},
		}
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.Anything).Return(evalResult, nil)
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "forbidden", response["error"])
		
		mockPolicyService.AssertExpectations(t)
	})
	
	t.Run("rate limit exceeded", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		// Mock policy evaluation with rate limit
		rateLimitConfig := &models.RateLimitConfig{
			RequestsPerMinute: 100,
		}
		evalResult := &policy.EvaluationResult{
			Allowed:         true,
			RateLimitConfig: rateLimitConfig,
		}
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.Anything).Return(evalResult, nil)
		
		// Mock rate limit check - exceeded
		rateLimitResult := &ratelimit.RateLimitResult{
			Allowed:           false,
			RequestsRemaining: 0,
			ResetAt:           time.Now().Add(30 * time.Second),
			ViolatedWindow:    ratelimit.WindowMinute,
			ViolationReason:   "exceeded 100 requests per minute",
		}
		mockRateLimitService.On("CheckLimit", mock.Anything, mock.Anything).Return(rateLimitResult, nil)
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, "rate_limit_exceeded", response["error"])
		
		mockPolicyService.AssertExpectations(t)
		mockRateLimitService.AssertExpectations(t)
	})
	
	t.Run("rate limit check fails", func(t *testing.T) {
		mockPolicyService := new(MockPolicyService)
		mockRateLimitService := new(MockRateLimitService)
		middleware := NewPolicyEnforcementMiddleware(mockPolicyService, mockRateLimitService, logger)
		
		// Mock policy evaluation with rate limit
		rateLimitConfig := &models.RateLimitConfig{
			RequestsPerMinute: 100,
		}
		evalResult := &policy.EvaluationResult{
			Allowed:         true,
			RateLimitConfig: rateLimitConfig,
		}
		
		mockPolicyService.On("Evaluate", mock.Anything, mock.Anything).Return(evalResult, nil)
		mockRateLimitService.On("CheckLimit", mock.Anything, mock.Anything).
			Return(nil, errors.New("rate limit error"))
		
		handler := middleware.EnforcePolicy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		ctx := context.WithValue(req.Context(), OrgIDKey, orgID)
		ctx = context.WithValue(ctx, AppIDKey, appID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		
		mockPolicyService.AssertExpectations(t)
		mockRateLimitService.AssertExpectations(t)
	})
}

func TestGetPolicyResultFromContext(t *testing.T) {
	t.Run("result exists in context", func(t *testing.T) {
		result := &policy.EvaluationResult{
			Allowed: true,
		}
		
		ctx := context.WithValue(context.Background(), "policy_evaluation_result", result)
		
		extracted := GetPolicyResultFromContext(ctx)
		assert.NotNil(t, extracted)
		assert.Equal(t, result, extracted)
	})
	
	t.Run("result does not exist in context", func(t *testing.T) {
		ctx := context.Background()
		
		extracted := GetPolicyResultFromContext(ctx)
		assert.Nil(t, extracted)
	})
	
	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "policy_evaluation_result", "wrong type")
		
		extracted := GetPolicyResultFromContext(ctx)
		assert.Nil(t, extracted)
	})
}

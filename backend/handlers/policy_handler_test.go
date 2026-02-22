package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/middleware"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Policy), args.Error(1)
}

func (m *MockPolicyRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Policy), args.Error(1)
}

func (m *MockPolicyRepository) GetByAppID(ctx context.Context, appID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, appID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Policy), args.Error(1)
}

func (m *MockPolicyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Policy), args.Error(1)
}

func (m *MockPolicyRepository) GetApplicablePolicies(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error) {
	args := m.Called(ctx, orgID, appID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Policy), args.Error(1)
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

func TestHandleListPolicies(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.New()
	
	t.Run("list org policies", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		policies := []*models.Policy{
			{
				ID:         uuid.New(),
				OrgID:      orgID,
				PolicyType: models.PolicyTypeRateLimit,
				Config:     json.RawMessage(`{"requests_per_minute": 100}`),
				Priority:   10,
				Enabled:    true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			{
				ID:         uuid.New(),
				OrgID:      orgID,
				PolicyType: models.PolicyTypeBudget,
				Config:     json.RawMessage(`{"max_daily_cost": 100.0}`),
				Priority:   5,
				Enabled:    true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
		}
		
		mockRepo.On("GetByOrgID", mock.Anything, orgID).Return(policies, nil)
		
		req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
		ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleListPolicies(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].([]interface{})
		assert.Len(t, data, 2)
		
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("missing org ID", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
		w := httptest.NewRecorder()
		
		handler.HandleListPolicies(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandleCreatePolicy(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.New()
	
	t.Run("successful creation", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(p *models.Policy) bool {
			return p.OrgID == orgID && p.PolicyType == models.PolicyTypeRateLimit
		})).Return(nil)
		
		reqBody := CreatePolicyRequest{
			PolicyType: models.PolicyTypeRateLimit,
			Config:     json.RawMessage(`{"requests_per_minute": 100}`),
			Priority:   10,
			Enabled:    true,
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/policies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleCreatePolicy(w, req)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, string(models.PolicyTypeRateLimit), data["policy_type"])
		assert.Equal(t, float64(10), data["priority"])
		assert.Equal(t, true, data["enabled"])
		
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("validation error - missing policy type", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		reqBody := CreatePolicyRequest{
			Config:   json.RawMessage(`{"requests_per_minute": 100}`),
			Priority: 10,
			Enabled:  true,
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPost, "/v1/policies", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleCreatePolicy(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleUpdatePolicy(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.New()
	policyID := uuid.New()
	
	t.Run("successful update", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		existingPolicy := &models.Policy{
			ID:         policyID,
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     json.RawMessage(`{"requests_per_minute": 100}`),
			Priority:   10,
			Enabled:    true,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		
		mockRepo.On("GetByID", mock.Anything, policyID).Return(existingPolicy, nil)
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(p *models.Policy) bool {
			return p.ID == policyID && p.Priority == 20
		})).Return(nil)
		
		newPriority := 20
		reqBody := UpdatePolicyRequest{
			Priority: &newPriority,
		}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPatch, "/v1/policies/"+policyID.String(), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		// Setup chi context for URL params
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", policyID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleUpdatePolicy(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("policy not found", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		mockRepo.On("GetByID", mock.Anything, policyID).Return(nil, errors.New("not found"))
		
		reqBody := UpdatePolicyRequest{}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPatch, "/v1/policies/"+policyID.String(), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", policyID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleUpdatePolicy(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("ownership mismatch", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		differentOrgID := uuid.New()
		existingPolicy := &models.Policy{
			ID:         policyID,
			OrgID:      differentOrgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     json.RawMessage(`{"requests_per_minute": 100}`),
			Priority:   10,
			Enabled:    true,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		
		mockRepo.On("GetByID", mock.Anything, policyID).Return(existingPolicy, nil)
		
		reqBody := UpdatePolicyRequest{}
		body, _ := json.Marshal(reqBody)
		
		req := httptest.NewRequest(http.MethodPatch, "/v1/policies/"+policyID.String(), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", policyID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleUpdatePolicy(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
		
		mockRepo.AssertExpectations(t)
	})
}

func TestHandleDeletePolicy(t *testing.T) {
	logger := zap.NewNop()
	orgID := uuid.New()
	policyID := uuid.New()
	
	t.Run("successful deletion", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		existingPolicy := &models.Policy{
			ID:         policyID,
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Config:     json.RawMessage(`{"requests_per_minute": 100}`),
			Priority:   10,
			Enabled:    true,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		
		mockRepo.On("GetByID", mock.Anything, policyID).Return(existingPolicy, nil)
		mockRepo.On("Delete", mock.Anything, policyID).Return(nil)
		
		req := httptest.NewRequest(http.MethodDelete, "/v1/policies/"+policyID.String(), nil)
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", policyID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleDeletePolicy(w, req)
		
		assert.Equal(t, http.StatusNoContent, w.Code)
		
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("policy not found", func(t *testing.T) {
		mockRepo := new(MockPolicyRepository)
		handler := NewPolicyHandler(mockRepo, logger)
		
		mockRepo.On("GetByID", mock.Anything, policyID).Return(nil, errors.New("not found"))
		
		req := httptest.NewRequest(http.MethodDelete, "/v1/policies/"+policyID.String(), nil)
		
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", policyID.String())
		ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
		ctx = context.WithValue(ctx, middleware.OrgIDKey, orgID)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		
		handler.HandleDeletePolicy(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		mockRepo.AssertExpectations(t)
	})
}

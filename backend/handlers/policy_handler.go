package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/middleware"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// CreatePolicyRequest represents a request to create a policy
type CreatePolicyRequest struct {
	PolicyType models.PolicyType   `json:"policy_type" validate:"required"`
	AppID      *uuid.UUID          `json:"app_id,omitempty"`
	UserID     *uuid.UUID          `json:"user_id,omitempty"`
	Config     json.RawMessage     `json:"config" validate:"required"`
	Priority   int                 `json:"priority" validate:"gte=0"`
	Enabled    bool                `json:"enabled"`
}

// UpdatePolicyRequest represents a request to update a policy
type UpdatePolicyRequest struct {
	Config   *json.RawMessage `json:"config,omitempty"`
	Priority *int             `json:"priority,omitempty" validate:"omitempty,gte=0"`
	Enabled  *bool            `json:"enabled,omitempty"`
}

// PolicyResponse represents a policy in API responses
type PolicyResponse struct {
	ID         uuid.UUID           `json:"id"`
	OrgID      uuid.UUID           `json:"org_id"`
	AppID      *uuid.UUID          `json:"app_id,omitempty"`
	UserID     *uuid.UUID          `json:"user_id,omitempty"`
	PolicyType models.PolicyType   `json:"policy_type"`
	Config     json.RawMessage     `json:"config"`
	Priority   int                 `json:"priority"`
	Enabled    bool                `json:"enabled"`
	CreatedAt  string              `json:"created_at"`
	UpdatedAt  string              `json:"updated_at"`
}

// PolicyService defines the interface for policy operations
type PolicyService interface {
	// CreatePolicy creates a new policy
	CreatePolicy(ctx context.Context, policy *models.Policy) error
	
	// GetPolicyByID retrieves a policy by ID
	GetPolicyByID(ctx context.Context, policyID, orgID uuid.UUID) (*models.Policy, error)
	
	// ListPolicies lists all policies for an organization/app/user
	ListPolicies(ctx context.Context, orgID uuid.UUID, appID, userID *uuid.UUID) ([]*models.Policy, error)
	
	// UpdatePolicy updates a policy
	UpdatePolicy(ctx context.Context, policy *models.Policy) error
	
	// DeletePolicy deletes a policy
	DeletePolicy(ctx context.Context, policyID, orgID uuid.UUID) error
}

// PolicyHandler handles policy-related HTTP requests
type PolicyHandler struct {
	policyRepo repositories.PolicyRepository
	logger     *zap.Logger
}

// NewPolicyHandler creates a new PolicyHandler
func NewPolicyHandler(policyRepo repositories.PolicyRepository, logger *zap.Logger) *PolicyHandler {
	return &PolicyHandler{
		policyRepo: policyRepo,
		logger:     logger,
	}
}

// HandleListPolicies handles GET /v1/policies
func (h *PolicyHandler) HandleListPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)
	
	// Extract tenant information
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		h.logger.Error("missing org ID in context")
		_ = utils.WriteUnauthorized(w, "Missing organization information")
		return
	}
	
	// Optional filters
	var appID, userID *uuid.UUID
	if appIDStr := r.URL.Query().Get("app_id"); appIDStr != "" {
		parsed, err := uuid.Parse(appIDStr)
		if err != nil {
			_ = utils.WriteBadRequest(w, "Invalid app_id format", nil)
			return
		}
		appID = &parsed
	}
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		parsed, err := uuid.Parse(userIDStr)
		if err != nil {
			_ = utils.WriteBadRequest(w, "Invalid user_id format", nil)
			return
		}
		userID = &parsed
	}
	
	// Fetch policies
	h.logger.Debug("listing policies",
		zap.String("request_id", requestID),
		zap.String("org_id", orgID.String()))
	
	var policies []*models.Policy
	var err error
	
	if userID != nil {
		policies, err = h.policyRepo.GetByUserID(ctx, *userID)
	} else if appID != nil {
		policies, err = h.policyRepo.GetByAppID(ctx, *appID)
	} else {
		policies, err = h.policyRepo.GetByOrgID(ctx, orgID)
	}
	
	if err != nil {
		h.logger.Error("failed to list policies",
			zap.String("request_id", requestID),
			zap.Error(err))
		_ = utils.WriteInternalServerError(w, "Failed to retrieve policies")
		return
	}
	
	// Convert to response format
	responses := make([]PolicyResponse, len(policies))
	for i, p := range policies {
		responses[i] = policyToResponse(p)
	}
	
	h.logger.Info("listed policies",
		zap.String("request_id", requestID),
		zap.Int("count", len(responses)))
	
	_ = utils.WriteOK(w, responses)
}

// HandleCreatePolicy handles POST /v1/policies
func (h *PolicyHandler) HandleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)
	
	// Extract tenant information
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		h.logger.Error("missing org ID in context")
		_ = utils.WriteUnauthorized(w, "Missing organization information")
		return
	}
	
	// Parse request body
	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("failed to parse request body",
			zap.String("request_id", requestID),
			zap.Error(err))
		_ = utils.WriteBadRequest(w, "Invalid request body", nil)
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		h.logger.Warn("request validation failed",
			zap.String("request_id", requestID),
			zap.Error(err))
		HandleValidationError(w, err, h.logger)
		return
	}
	
	// Create policy
	policy := models.NewPolicy(orgID, req.PolicyType, req.Config, req.Priority)
	policy.AppID = req.AppID
	policy.UserID = req.UserID
	policy.Enabled = req.Enabled
	
	h.logger.Debug("creating policy",
		zap.String("request_id", requestID),
		zap.String("org_id", orgID.String()),
		zap.String("policy_type", string(req.PolicyType)))
	
	if err := h.policyRepo.Create(ctx, policy); err != nil {
		h.logger.Error("failed to create policy",
			zap.String("request_id", requestID),
			zap.Error(err))
		_ = utils.WriteInternalServerError(w, "Failed to create policy")
		return
	}
	
	h.logger.Info("policy created",
		zap.String("request_id", requestID),
		zap.String("policy_id", policy.ID.String()),
		zap.String("policy_type", string(policy.PolicyType)))
	
	_ = utils.WriteCreated(w, policyToResponse(policy))
}

// HandleUpdatePolicy handles PATCH /v1/policies/{id}
func (h *PolicyHandler) HandleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)
	
	// Extract tenant information
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		h.logger.Error("missing org ID in context")
		_ = utils.WriteUnauthorized(w, "Missing organization information")
		return
	}
	
	// Parse policy ID
	policyIDStr := chi.URLParam(r, "id")
	policyID, err := uuid.Parse(policyIDStr)
	if err != nil {
		_ = utils.WriteBadRequest(w, "Invalid policy ID format", nil)
		return
	}
	
	// Parse request body
	var req UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("failed to parse request body",
			zap.String("request_id", requestID),
			zap.Error(err))
		_ = utils.WriteBadRequest(w, "Invalid request body", nil)
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(&req); err != nil {
		h.logger.Warn("request validation failed",
			zap.String("request_id", requestID),
			zap.Error(err))
		HandleValidationError(w, err, h.logger)
		return
	}
	
	// Fetch existing policy
	h.logger.Debug("updating policy",
		zap.String("request_id", requestID),
		zap.String("policy_id", policyID.String()))
	
	policy, err := h.policyRepo.GetByID(ctx, policyID)
	if err != nil {
		h.logger.Error("failed to fetch policy",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.Error(err))
		_ = utils.WriteNotFound(w, "Policy not found")
		return
	}
	
	// Verify ownership
	if policy.OrgID != orgID {
		h.logger.Warn("policy ownership mismatch",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.String("expected_org_id", orgID.String()),
			zap.String("actual_org_id", policy.OrgID.String()))
		_ = utils.WriteForbidden(w, "Access denied to this policy")
		return
	}
	
	// Update fields
	if req.Config != nil {
		policy.Config = *req.Config
	}
	if req.Priority != nil {
		policy.Priority = *req.Priority
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	
	// Save changes
	if err := h.policyRepo.Update(ctx, policy); err != nil {
		h.logger.Error("failed to update policy",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.Error(err))
		_ = utils.WriteInternalServerError(w, "Failed to update policy")
		return
	}
	
	h.logger.Info("policy updated",
		zap.String("request_id", requestID),
		zap.String("policy_id", policyID.String()))
	
	_ = utils.WriteOK(w, policyToResponse(policy))
}

// HandleDeletePolicy handles DELETE /v1/policies/{id}
func (h *PolicyHandler) HandleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestIDFromContext(ctx)
	
	// Extract tenant information
	orgID := middleware.GetOrgIDFromContext(ctx)
	if orgID == uuid.Nil {
		h.logger.Error("missing org ID in context")
		_ = utils.WriteUnauthorized(w, "Missing organization information")
		return
	}
	
	// Parse policy ID
	policyIDStr := chi.URLParam(r, "id")
	policyID, err := uuid.Parse(policyIDStr)
	if err != nil {
		_ = utils.WriteBadRequest(w, "Invalid policy ID format", nil)
		return
	}
	
	h.logger.Debug("deleting policy",
		zap.String("request_id", requestID),
		zap.String("policy_id", policyID.String()))
	
	// Fetch existing policy to verify ownership
	policy, err := h.policyRepo.GetByID(ctx, policyID)
	if err != nil {
		h.logger.Error("failed to fetch policy",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.Error(err))
		_ = utils.WriteNotFound(w, "Policy not found")
		return
	}
	
	// Verify ownership
	if policy.OrgID != orgID {
		h.logger.Warn("policy ownership mismatch",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.String("expected_org_id", orgID.String()),
			zap.String("actual_org_id", policy.OrgID.String()))
		_ = utils.WriteForbidden(w, "Access denied to this policy")
		return
	}
	
	// Delete policy
	if err := h.policyRepo.Delete(ctx, policyID); err != nil {
		h.logger.Error("failed to delete policy",
			zap.String("request_id", requestID),
			zap.String("policy_id", policyID.String()),
			zap.Error(err))
		_ = utils.WriteInternalServerError(w, "Failed to delete policy")
		return
	}
	
	h.logger.Info("policy deleted",
		zap.String("request_id", requestID),
		zap.String("policy_id", policyID.String()))
	
	utils.WriteNoContent(w)
}

// policyToResponse converts a Policy model to a PolicyResponse
func policyToResponse(p *models.Policy) PolicyResponse {
	return PolicyResponse{
		ID:         p.ID,
		OrgID:      p.OrgID,
		AppID:      p.AppID,
		UserID:     p.UserID,
		PolicyType: p.PolicyType,
		Config:     p.Config,
		Priority:   p.Priority,
		Enabled:    p.Enabled,
		CreatedAt:  p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

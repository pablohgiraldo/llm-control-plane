package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
	"github.com/upb/llm-control-plane/backend/repositories"
	"go.uber.org/zap"
)

// EvaluationRequest represents a request to evaluate policies
type EvaluationRequest struct {
	OrgID    uuid.UUID
	AppID    uuid.UUID
	UserID   *uuid.UUID
	Provider string
	Model    string
	Prompt   string
}

// EvaluationResult represents the result of policy evaluation
type EvaluationResult struct {
	Allowed          bool
	AppliedPolicies  []*models.Policy
	Violations       []PolicyViolation
	RateLimitConfig  *models.RateLimitConfig
	BudgetConfig     *models.BudgetConfig
	RoutingConfig    *models.RoutingConfig
	PIIConfig        *models.PIIConfig
	InjectionConfig  *models.InjectionGuardConfig
	RAGConfig        *models.RAGConfig
}

// PolicyViolation represents a policy violation
type PolicyViolation struct {
	PolicyID   uuid.UUID
	PolicyType models.PolicyType
	Reason     string
	Details    interface{}
}

// PolicyService handles policy evaluation and management
type PolicyService struct {
	policyRepo repositories.PolicyRepository
	cache      *PolicyCache
	logger     *zap.Logger
}

// NewPolicyService creates a new PolicyService instance
func NewPolicyService(policyRepo repositories.PolicyRepository, cache *PolicyCache, logger *zap.Logger) *PolicyService {
	return &PolicyService{
		policyRepo: policyRepo,
		cache:      cache,
		logger:     logger,
	}
}

// Evaluate evaluates all applicable policies for a request
func (s *PolicyService) Evaluate(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	// Fetch applicable policies (with caching)
	policies, err := s.getApplicablePolicies(ctx, req.OrgID, req.AppID, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch policies: %w", err)
	}

	// Merge policies by type with priority
	mergedPolicies := s.mergePolicies(policies)

	// Build evaluation result
	result := &EvaluationResult{
		Allowed:         true,
		AppliedPolicies: mergedPolicies,
		Violations:      make([]PolicyViolation, 0),
	}

	// Extract configurations by type
	for _, policy := range mergedPolicies {
		if !policy.Enabled {
			continue
		}

		switch policy.PolicyType {
		case models.PolicyTypeRateLimit:
			var config models.RateLimitConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal rate limit config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.RateLimitConfig = &config

		case models.PolicyTypeBudget:
			var config models.BudgetConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal budget config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.BudgetConfig = &config

		case models.PolicyTypeRouting:
			var config models.RoutingConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal routing config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.RoutingConfig = &config

		case models.PolicyTypePII:
			var config models.PIIConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal PII config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.PIIConfig = &config

		case models.PolicyTypeInjection:
			var config models.InjectionGuardConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal injection guard config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.InjectionConfig = &config

		case models.PolicyTypeRAG:
			var config models.RAGConfig
			if err := json.Unmarshal(policy.Config, &config); err != nil {
				s.logger.Error("failed to unmarshal RAG config",
					zap.Error(err),
					zap.String("policy_id", policy.ID.String()))
				continue
			}
			result.RAGConfig = &config
		}
	}

	return result, nil
}

// getApplicablePolicies fetches applicable policies with caching
func (s *PolicyService) getApplicablePolicies(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error) {
	// Try cache first
	cacheKey := CacheKey{
		OrgID:  orgID,
		AppID:  appID,
		UserID: userID,
	}

	if cached := s.cache.GetPolicies(cacheKey); cached != nil {
		s.logger.Debug("cache hit for policies",
			zap.String("org_id", orgID.String()),
			zap.String("app_id", appID.String()))
		return cached, nil
	}

	// Fetch from database hierarchically
	policies, err := s.fetchPoliciesHierarchical(ctx, orgID, appID, userID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	s.cache.SetPolicies(cacheKey, policies)

	s.logger.Debug("cache miss for policies, fetched from database",
		zap.String("org_id", orgID.String()),
		zap.String("app_id", appID.String()),
		zap.Int("count", len(policies)))

	return policies, nil
}

// fetchPoliciesHierarchical fetches policies in hierarchical order:
// org-level → app-level → user-level
func (s *PolicyService) fetchPoliciesHierarchical(ctx context.Context, orgID, appID uuid.UUID, userID *uuid.UUID) ([]*models.Policy, error) {
	policies := make([]*models.Policy, 0)

	// 1. Fetch org-level policies
	orgPolicies, err := s.policyRepo.GetByOrgID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch org policies: %w", err)
	}

	// Filter org-level only (AppID and UserID are null)
	for _, p := range orgPolicies {
		if p.AppID == nil && p.UserID == nil {
			policies = append(policies, p)
		}
	}

	// 2. Fetch app-level policies
	appPolicies, err := s.policyRepo.GetByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app policies: %w", err)
	}

	// Filter app-level only (UserID is null)
	for _, p := range appPolicies {
		if p.UserID == nil {
			policies = append(policies, p)
		}
	}

	// 3. Fetch user-level policies if userID is provided
	if userID != nil {
		userPolicies, err := s.policyRepo.GetByUserID(ctx, *userID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch user policies: %w", err)
		}
		policies = append(policies, userPolicies...)
	}

	return policies, nil
}

// mergePolicies merges policies by type, prioritizing higher-level (user > app > org)
// and within the same level, higher priority values take precedence
func (s *PolicyService) mergePolicies(policies []*models.Policy) []*models.Policy {
	// Group policies by type
	policyMap := make(map[models.PolicyType][]*models.Policy)
	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		policyMap[p.PolicyType] = append(policyMap[p.PolicyType], p)
	}

	// For each type, select the highest priority policy
	merged := make([]*models.Policy, 0)
	for policyType, typePolicies := range policyMap {
		if len(typePolicies) == 0 {
			continue
		}

		// Sort by priority: user-level > app-level > org-level, then by priority value
		sort.Slice(typePolicies, func(i, j int) bool {
			pi := typePolicies[i]
			pj := typePolicies[j]

			// Calculate hierarchy level (user=3, app=2, org=1)
			levelI := s.getPolicyLevel(pi)
			levelJ := s.getPolicyLevel(pj)

			if levelI != levelJ {
				return levelI > levelJ // Higher level wins
			}

			// Same level, compare priority
			return pi.Priority > pj.Priority // Higher priority wins
		})

		// Take the highest priority policy for this type
		selectedPolicy := typePolicies[0]
		merged = append(merged, selectedPolicy)

		s.logger.Debug("selected policy",
			zap.String("type", string(policyType)),
			zap.String("policy_id", selectedPolicy.ID.String()),
			zap.Int("priority", selectedPolicy.Priority),
			zap.Int("level", s.getPolicyLevel(selectedPolicy)))
	}

	return merged
}

// getPolicyLevel returns the hierarchy level of a policy
// user=3, app=2, org=1
func (s *PolicyService) getPolicyLevel(p *models.Policy) int {
	if p.UserID != nil {
		return 3
	}
	if p.AppID != nil {
		return 2
	}
	return 1
}

// InvalidateCache invalidates cache for specific keys
func (s *PolicyService) InvalidateCache(orgID, appID uuid.UUID, userID *uuid.UUID) {
	cacheKey := CacheKey{
		OrgID:  orgID,
		AppID:  appID,
		UserID: userID,
	}
	s.cache.Invalidate(cacheKey)
	s.logger.Debug("invalidated cache",
		zap.String("org_id", orgID.String()),
		zap.String("app_id", appID.String()))
}

// InvalidateCacheForOrg invalidates all cache entries for an organization
func (s *PolicyService) InvalidateCacheForOrg(orgID uuid.UUID) {
	s.cache.InvalidateOrg(orgID)
	s.logger.Debug("invalidated cache for org",
		zap.String("org_id", orgID.String()))
}

// InvalidateCacheForApp invalidates all cache entries for an application
func (s *PolicyService) InvalidateCacheForApp(orgID, appID uuid.UUID) {
	s.cache.InvalidateApp(orgID, appID)
	s.logger.Debug("invalidated cache for app",
		zap.String("org_id", orgID.String()),
		zap.String("app_id", appID.String()))
}

// InvalidateCacheForUser invalidates all cache entries for a user
func (s *PolicyService) InvalidateCacheForUser(orgID, appID uuid.UUID, userID uuid.UUID) {
	s.cache.InvalidateUser(orgID, appID, userID)
	s.logger.Debug("invalidated cache for user",
		zap.String("org_id", orgID.String()),
		zap.String("app_id", appID.String()),
		zap.String("user_id", userID.String()))
}

// GetCacheStats returns cache statistics
func (s *PolicyService) GetCacheStats() CacheStats {
	return s.cache.Stats()
}

// StartCacheCleanup starts a background worker to clean up expired cache entries
func (s *PolicyService) StartCacheCleanup(interval time.Duration, stopCh <-chan struct{}) {
	s.cache.StartCleanupWorker(interval, stopCh)
	s.logger.Info("started cache cleanup worker",
		zap.Duration("interval", interval))
}

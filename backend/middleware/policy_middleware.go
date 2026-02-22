package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/services/policy"
	"github.com/upb/llm-control-plane/backend/services/ratelimit"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// PolicyEvaluator defines the interface for policy evaluation
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, req policy.EvaluationRequest) (*policy.EvaluationResult, error)
}

// RateLimitChecker defines the interface for rate limit checking
type RateLimitChecker interface {
	CheckLimit(ctx context.Context, req ratelimit.RateLimitRequest) (*ratelimit.RateLimitResult, error)
	RecordRequest(ctx context.Context, req ratelimit.RateLimitRequest) error
}

// PolicyEnforcementMiddleware provides policy enforcement functionality
type PolicyEnforcementMiddleware struct {
	policyService    PolicyEvaluator
	rateLimitService RateLimitChecker
	logger           *zap.Logger
}

// NewPolicyEnforcementMiddleware creates a new PolicyEnforcementMiddleware
func NewPolicyEnforcementMiddleware(
	policyService PolicyEvaluator,
	rateLimitService RateLimitChecker,
	logger *zap.Logger,
) *PolicyEnforcementMiddleware {
	return &PolicyEnforcementMiddleware{
		policyService:    policyService,
		rateLimitService: rateLimitService,
		logger:           logger,
	}
}

// EnforcePolicy is a middleware that evaluates and enforces policies
// This should be called after auth and tenant extraction middleware
func (m *PolicyEnforcementMiddleware) EnforcePolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := GetRequestIDFromContext(ctx)
		
		// Extract tenant information from context
		orgID := GetOrgIDFromContext(ctx)
		appID := GetAppIDFromContext(ctx)
		userID := GetUserIDFromContext(ctx)
		
		if orgID == uuid.Nil || appID == uuid.Nil {
			m.logger.Error("missing tenant information in context",
				zap.String("request_id", requestID))
			_ = utils.WriteUnauthorized(w, "Missing tenant information")
			return
		}
		
		// Build evaluation request
		evalReq := policy.EvaluationRequest{
			OrgID:  orgID,
			AppID:  appID,
			UserID: userID,
			// Note: For a full implementation, you might want to extract
			// provider, model, and prompt from the request body
			// For now, we'll evaluate policies without these details
		}
		
		m.logger.Debug("evaluating policies",
			zap.String("request_id", requestID),
			zap.String("org_id", orgID.String()),
			zap.String("app_id", appID.String()))
		
		// Evaluate policies
		result, err := m.policyService.Evaluate(ctx, evalReq)
		if err != nil {
			m.logger.Error("failed to evaluate policies",
				zap.String("request_id", requestID),
				zap.Error(err))
			_ = utils.WriteInternalServerError(w, "Failed to evaluate policies")
			return
		}
		
		// Check if request is allowed
		if !result.Allowed {
			m.logger.Warn("request blocked by policy",
				zap.String("request_id", requestID),
				zap.Int("violations", len(result.Violations)))
			
			// Return first violation reason
			if len(result.Violations) > 0 {
				violation := result.Violations[0]
				_ = utils.WriteForbidden(w, violation.Reason)
				return
			}
			
			_ = utils.WriteForbidden(w, "Request blocked by policy")
			return
		}
		
		// Check rate limits if configured
		if result.RateLimitConfig != nil {
			rateLimitReq := ratelimit.RateLimitRequest{
				OrgID:  orgID,
				AppID:  appID,
				UserID: userID,
				Config: result.RateLimitConfig,
				// Note: TokensUsed would need to be calculated from the request
				TokensUsed: 0,
			}
			
			rateLimitResult, err := m.rateLimitService.CheckLimit(ctx, rateLimitReq)
			if err != nil {
				m.logger.Error("failed to check rate limit",
					zap.String("request_id", requestID),
					zap.Error(err))
				_ = utils.WriteInternalServerError(w, "Failed to check rate limit")
				return
			}
			
			if !rateLimitResult.Allowed {
				m.logger.Warn("request blocked by rate limit",
					zap.String("request_id", requestID),
					zap.String("window", string(rateLimitResult.ViolatedWindow)),
					zap.String("reason", rateLimitResult.ViolationReason))
				
				details := map[string]interface{}{
					"window":             string(rateLimitResult.ViolatedWindow),
					"requests_remaining": rateLimitResult.RequestsRemaining,
					"reset_at":           rateLimitResult.ResetAt.Format("2006-01-02T15:04:05Z07:00"),
				}
				
				_ = utils.WriteTooManyRequests(w, rateLimitResult.ViolationReason, details)
				return
			}
			
			// Add rate limit headers
			w.Header().Set("X-RateLimit-Remaining", string(rune(rateLimitResult.RequestsRemaining)))
			w.Header().Set("X-RateLimit-Reset", rateLimitResult.ResetAt.Format("2006-01-02T15:04:05Z07:00"))
		}
		
		m.logger.Debug("policy enforcement passed",
			zap.String("request_id", requestID),
			zap.Int("policies_applied", len(result.AppliedPolicies)))
		
		// Store evaluation result in context for use by handlers
		ctx = context.WithValue(ctx, "policy_evaluation_result", result)
		
		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetPolicyResultFromContext retrieves the policy evaluation result from context
func GetPolicyResultFromContext(ctx context.Context) *policy.EvaluationResult {
	if val := ctx.Value("policy_evaluation_result"); val != nil {
		if result, ok := val.(*policy.EvaluationResult); ok {
			return result
		}
	}
	return nil
}

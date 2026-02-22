package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

// TokenValidator defines the interface for validating JWT tokens
type TokenValidator interface {
	// ValidateToken validates a JWT token and returns claims
	ValidateToken(ctx context.Context, token string) (*Claims, error)
}

// AuthMiddleware provides authentication middleware functionality
type AuthMiddleware struct {
	validator TokenValidator
	logger    *zap.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(validator TokenValidator, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		validator: validator,
		logger:    logger,
	}
}

// authTokenCookieName is the cookie name for JWT tokens (Authorization header takes precedence)
// sessionCookieName is set by auth handler after OAuth callback
const authTokenCookieName = "auth_token"
const sessionCookieName = "session"

// RequireAuth is a middleware that requires a valid JWT token
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := GetRequestIDFromContext(ctx)
		
		// Extract token from cookie ("auth_token") or Authorization header ("Bearer TOKEN")
		token := extractToken(r)
		if token == "" {
			m.logger.Warn("missing token",
				zap.String("request_id", requestID))
			_ = utils.WriteUnauthorized(w, "Missing or invalid authorization")
			return
		}
		
		// Validate token
		claims, err := m.validator.ValidateToken(ctx, token)
		if err != nil {
			m.logger.Warn("token validation failed",
				zap.String("request_id", requestID),
				zap.Error(err))
			_ = utils.WriteUnauthorized(w, "Invalid or expired token")
			return
		}
		
		// Add claims to context
		ctx = WithClaims(ctx, claims)
		
		m.logger.Debug("authentication successful",
			zap.String("request_id", requestID),
			zap.String("sub", claims.Sub),
			zap.String("email", claims.Email))
		
		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ExtractTenant is a middleware that extracts tenant information from claims
// This should be called after RequireAuth
func (m *AuthMiddleware) ExtractTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := GetRequestIDFromContext(ctx)
		
		// Get claims from context
		claims := GetClaimsFromContext(ctx)
		if claims == nil {
			m.logger.Error("claims not found in context",
				zap.String("request_id", requestID))
			_ = utils.WriteUnauthorized(w, "Authentication required")
			return
		}
		
		// Parse and validate tenant IDs (OrgID from custom:tenantId, AppID from custom:app_id optional)
		orgID, err := uuid.Parse(claims.OrgID)
		if err != nil {
			m.logger.Error("invalid org_id (custom:tenantId) in claims",
				zap.String("request_id", requestID),
				zap.String("org_id", claims.OrgID),
				zap.Error(err))
			_ = utils.WriteForbidden(w, "Invalid organization ID")
			return
		}

		// AppID is optional (not in actions-aws-auth schema); use uuid.Nil when empty
		var appID uuid.UUID
		if claims.AppID != "" {
			parsedAppID, err := uuid.Parse(claims.AppID)
			if err != nil {
				m.logger.Error("invalid app_id in claims",
					zap.String("request_id", requestID),
					zap.String("app_id", claims.AppID),
					zap.Error(err))
				_ = utils.WriteForbidden(w, "Invalid application ID")
				return
			}
			appID = parsedAppID
		}
		
		// Add tenant information to context
		ctx = WithOrgID(ctx, orgID)
		ctx = WithAppID(ctx, appID)
		
		// Parse user ID if present
		if claims.UserID != "" {
			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				m.logger.Warn("invalid user_id in claims",
					zap.String("request_id", requestID),
					zap.String("user_id", claims.UserID),
					zap.Error(err))
			} else {
				ctx = WithUserID(ctx, &userID)
			}
		}
		
		m.logger.Debug("tenant information extracted",
			zap.String("request_id", requestID),
			zap.String("org_id", orgID.String()),
			zap.String("app_id", appID.String()))
		
		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole is a middleware that requires a specific role
func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID := GetRequestIDFromContext(ctx)
			
			// Get claims from context
			claims := GetClaimsFromContext(ctx)
			if claims == nil {
				m.logger.Error("claims not found in context",
					zap.String("request_id", requestID))
				_ = utils.WriteUnauthorized(w, "Authentication required")
				return
			}
			
			// Check if user has the required role
			hasRole := false
			for _, group := range claims.Groups {
				if group == role {
					hasRole = true
					break
				}
			}
			
			if !hasRole {
				m.logger.Warn("insufficient permissions",
					zap.String("request_id", requestID),
					zap.String("required_role", role),
					zap.Strings("user_groups", claims.Groups))
				_ = utils.WriteForbidden(w, "Insufficient permissions")
				return
			}
			
			m.logger.Debug("role check passed",
				zap.String("request_id", requestID),
				zap.String("required_role", role))
			
			// Call next handler
			next.ServeHTTP(w, r)
		})
	}
}

// extractToken extracts JWT from cookie ("auth_token") or Authorization header ("Bearer TOKEN").
// Authorization header takes precedence when both are present.
func extractToken(r *http.Request) string {
	// Try Authorization header first
	if token := extractBearerToken(r); token != "" {
		return token
	}
	// Fall back to auth_token or session cookie (session is set by OAuth callback)
	for _, name := range []string{authTokenCookieName, sessionCookieName} {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}

// extractBearerToken extracts the Bearer token from the Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	
	// Check if it starts with "Bearer "
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	
	return strings.TrimSpace(parts[1])
}

package middleware

import (
	"context"

	"github.com/google/uuid"
)

// Context key type to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	
	// ClaimsKey is the context key for JWT claims
	ClaimsKey contextKey = "claims"
	
	// OrgIDKey is the context key for organization ID
	OrgIDKey contextKey = "org_id"
	
	// AppIDKey is the context key for application ID
	AppIDKey contextKey = "app_id"
	
	// UserIDKey is the context key for user ID
	UserIDKey contextKey = "user_id"
)

// Claims represents JWT claims extracted from the token
type Claims struct {
	Sub           string   `json:"sub"`            // Subject (user ID in Cognito)
	Email         string   `json:"email"`          
	EmailVerified bool     `json:"email_verified"` 
	Groups        []string `json:"cognito:groups"` 
	OrgID         string   `json:"custom:org_id"`  
	AppID         string   `json:"custom:app_id"`  
	UserID        string   `json:"custom:user_id"` 
	Iss           string   `json:"iss"`            // Issuer
	Exp           int64    `json:"exp"`            // Expiration
	Iat           int64    `json:"iat"`            // Issued at
}

// GetRequestIDFromContext retrieves the request ID from context
func GetRequestIDFromContext(ctx context.Context) string {
	if val := ctx.Value(RequestIDKey); val != nil {
		if requestID, ok := val.(string); ok {
			return requestID
		}
	}
	return ""
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetClaimsFromContext retrieves JWT claims from context
func GetClaimsFromContext(ctx context.Context) *Claims {
	if val := ctx.Value(ClaimsKey); val != nil {
		if claims, ok := val.(*Claims); ok {
			return claims
		}
	}
	return nil
}

// WithClaims adds JWT claims to the context
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}

// GetOrgIDFromContext retrieves the organization ID from context
func GetOrgIDFromContext(ctx context.Context) uuid.UUID {
	if val := ctx.Value(OrgIDKey); val != nil {
		if orgID, ok := val.(uuid.UUID); ok {
			return orgID
		}
	}
	return uuid.Nil
}

// WithOrgID adds an organization ID to the context
func WithOrgID(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, OrgIDKey, orgID)
}

// GetAppIDFromContext retrieves the application ID from context
func GetAppIDFromContext(ctx context.Context) uuid.UUID {
	if val := ctx.Value(AppIDKey); val != nil {
		if appID, ok := val.(uuid.UUID); ok {
			return appID
		}
	}
	return uuid.Nil
}

// WithAppID adds an application ID to the context
func WithAppID(ctx context.Context, appID uuid.UUID) context.Context {
	return context.WithValue(ctx, AppIDKey, appID)
}

// GetUserIDFromContext retrieves the user ID from context
func GetUserIDFromContext(ctx context.Context) *uuid.UUID {
	if val := ctx.Value(UserIDKey); val != nil {
		if userID, ok := val.(*uuid.UUID); ok {
			return userID
		}
	}
	return nil
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID *uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

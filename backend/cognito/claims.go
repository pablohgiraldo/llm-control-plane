package cognito

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrMissingClaim is returned when a required claim is missing
	ErrMissingClaim = errors.New("missing required claim")

	// ErrInvalidClaimType is returned when a claim has an unexpected type
	ErrInvalidClaimType = errors.New("invalid claim type")
)

// ExtractClaims extracts and parses claims from a JWT token without validation
// This is useful when you already have a validated token and just need the claims
func ExtractClaims(tokenString string) (*ParsedClaims, error) {
	// Parse without validation
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	
	claims := &Claims{}
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return parseClaims(claims)
}

// ExtractClaimsFromValidatedToken extracts claims from an already validated jwt.Token
func ExtractClaimsFromValidatedToken(token *jwt.Token) (*ParsedClaims, error) {
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	return parseClaims(claims)
}

// parseClaims converts Claims to ParsedClaims with proper type conversions
func parseClaims(claims *Claims) (*ParsedClaims, error) {
	// Parse required Sub (user ID)
	if claims.Sub == "" {
		return nil, fmt.Errorf("%w: sub", ErrMissingClaim)
	}
	sub, err := uuid.Parse(claims.Sub)
	if err != nil {
		return nil, fmt.Errorf("invalid sub UUID: %w", err)
	}

	// Parse required OrgID (from custom:tenantId in actions-aws-auth)
	if claims.OrgID == "" {
		return nil, fmt.Errorf("%w: custom:tenantId", ErrMissingClaim)
	}
	orgID, err := uuid.Parse(claims.OrgID)
	if err != nil {
		return nil, fmt.Errorf("invalid custom:tenantId UUID: %w", err)
	}

	// Parse optional AppID
	var appID *uuid.UUID
	if claims.AppID != "" {
		parsedAppID, err := uuid.Parse(claims.AppID)
		if err != nil {
			return nil, fmt.Errorf("invalid app_id UUID: %w", err)
		}
		appID = &parsedAppID
	}

	// Build parsed claims
	parsed := &ParsedClaims{
		Sub:          sub,
		Email:        claims.Email,
		OrgID:        orgID,
		AppID:        appID,
		Role:         claims.Role,
		EmailVerified: claims.EmailVerified,
		Username:     claims.CognitoUsername,
	}
	
	// Set time fields if available
	if claims.IssuedAt != nil {
		parsed.IssuedAt = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		parsed.ExpiresAt = claims.ExpiresAt.Time
	}

	return parsed, nil
}

// ExtractOrgID extracts only the org ID from a token (fast path)
func ExtractOrgID(tokenString string) (uuid.UUID, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	
	claims := &Claims{}
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims.OrgID == "" {
		return uuid.Nil, fmt.Errorf("%w: custom:tenantId", ErrMissingClaim)
	}

	orgID, err := uuid.Parse(claims.OrgID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid custom:tenantId UUID: %w", err)
	}

	return orgID, nil
}

// ExtractAppID extracts only the app ID from a token (fast path)
func ExtractAppID(tokenString string) (*uuid.UUID, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	
	claims := &Claims{}
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims.AppID == "" {
		return nil, nil // AppID is optional
	}

	appID, err := uuid.Parse(claims.AppID)
	if err != nil {
		return nil, fmt.Errorf("invalid app_id UUID: %w", err)
	}

	return &appID, nil
}

// ExtractRole extracts only the role from a token (fast path)
func ExtractRole(tokenString string) (string, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	
	claims := &Claims{}
	_, _, err := parser.ParseUnverified(tokenString, claims)
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	return claims.Role, nil
}

// ValidateCustomClaims validates that all required custom claims are present
func ValidateCustomClaims(claims *Claims) error {
	if claims.OrgID == "" {
		return fmt.Errorf("%w: custom:tenantId", ErrMissingClaim)
	}

	// Validate OrgID format (tenantId from actions-aws-auth)
	if _, err := uuid.Parse(claims.OrgID); err != nil {
		return fmt.Errorf("invalid custom:tenantId format: %w", err)
	}

	// Validate AppID format if present (optional; not in actions-aws-auth schema)
	if claims.AppID != "" {
		if _, err := uuid.Parse(claims.AppID); err != nil {
			return fmt.Errorf("invalid custom:app_id format: %w", err)
		}
	}

	// Role (from custom:userRole) is optional but should be one of known values if present
	if claims.Role != "" {
		validRoles := map[string]bool{
			"admin":     true,
			"developer": true,
			"user":      true,
			"viewer":    true,
		}
		if !validRoles[claims.Role] {
			return fmt.Errorf("invalid custom:userRole value: %s", claims.Role)
		}
	}

	return nil
}

// GetUserContext creates a user context from parsed claims
// Useful for passing user information through the request pipeline
type UserContext struct {
	UserID        uuid.UUID
	Email         string
	OrgID         uuid.UUID
	AppID         *uuid.UUID
	Role          string
	EmailVerified bool
	Username      string
}

// ToUserContext converts ParsedClaims to UserContext
func (p *ParsedClaims) ToUserContext() *UserContext {
	return &UserContext{
		UserID:        p.Sub,
		Email:         p.Email,
		OrgID:         p.OrgID,
		AppID:         p.AppID,
		Role:          p.Role,
		EmailVerified: p.EmailVerified,
		Username:      p.Username,
	}
}

// IsAdmin checks if the user has admin role
func (u *UserContext) IsAdmin() bool {
	return u.Role == "admin"
}

// IsDeveloper checks if the user has developer role or higher
func (u *UserContext) IsDeveloper() bool {
	return u.Role == "admin" || u.Role == "developer"
}

// HasRole checks if the user has a specific role
func (u *UserContext) HasRole(role string) bool {
	return u.Role == role
}

// HasAnyRole checks if the user has any of the specified roles
func (u *UserContext) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.Role == role {
			return true
		}
	}
	return false
}

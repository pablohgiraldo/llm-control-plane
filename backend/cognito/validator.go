package cognito

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken is returned when the token is invalid
	ErrInvalidToken = errors.New("invalid token")

	// ErrTokenExpired is returned when the token has expired
	ErrTokenExpired = errors.New("token expired")

	// ErrInvalidIssuer is returned when the token issuer is invalid
	ErrInvalidIssuer = errors.New("invalid issuer")

	// ErrInvalidAudience is returned when the token audience is invalid
	ErrInvalidAudience = errors.New("invalid audience")

	// ErrJWKSFetchFailed is returned when JWKS fetching fails
	ErrJWKSFetchFailed = errors.New("failed to fetch JWKS")
)

// JWKS represents the JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Claims represents the custom claims in the JWT token
type Claims struct {
	jwt.RegisteredClaims
	Sub          string `json:"sub"`
	Email        string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	TokenUse     string `json:"token_use"`
	AuthTime     int64  `json:"auth_time"`
	CognitoUsername string `json:"cognito:username"`
	
	// Custom claims (aligned with actions-aws-auth: tenantId, userRole; app_id optional for backward compat)
	OrgID  string `json:"custom:tenantId"`   // tenantId from actions-aws-auth maps to OrgID
	AppID  string `json:"custom:app_id"`    // optional; not in actions-aws-auth schema
	Role   string `json:"custom:userRole"`  // userRole from actions-aws-auth maps to Role
}

// ParsedClaims represents parsed and validated claims
type ParsedClaims struct {
	Sub          uuid.UUID
	Email        string
	OrgID        uuid.UUID
	AppID        *uuid.UUID // Optional - may be nil
	Role         string
	EmailVerified bool
	Username     string
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

// CognitoValidator validates JWT tokens from AWS Cognito
type CognitoValidator struct {
	region       string
	userPoolID   string
	clientID     string
	jwksURL      string
	httpClient   *http.Client
	
	// Cache for JWKS
	jwksCache    *JWKS
	jwksCacheExp time.Time
	jwksCacheTTL time.Duration
	cacheMu      sync.RWMutex
	
	// Cache for parsed public keys
	keyCache map[string]*rsa.PublicKey
	keyCacheMu sync.RWMutex
}

// Config holds configuration for CognitoValidator
type Config struct {
	Region     string
	UserPoolID string
	ClientID   string
	CacheTTL   time.Duration
	HTTPTimeout time.Duration
}

// NewCognitoValidator creates a new Cognito JWT validator
func NewCognitoValidator(config Config) *CognitoValidator {
	if config.CacheTTL == 0 {
		config.CacheTTL = 1 * time.Hour
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 10 * time.Second
	}

	jwksURL := fmt.Sprintf(
		"https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json",
		config.Region,
		config.UserPoolID,
	)

	return &CognitoValidator{
		region:       config.Region,
		userPoolID:   config.UserPoolID,
		clientID:     config.ClientID,
		jwksURL:      jwksURL,
		jwksCacheTTL: config.CacheTTL,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
		keyCache: make(map[string]*rsa.PublicKey),
	}
}

// ValidateToken validates a JWT token and returns parsed claims
func (v *CognitoValidator) ValidateToken(ctx context.Context, tokenString string) (*ParsedClaims, error) {
	// Parse the token without validation first to get the kid
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the kid from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found")
		}

		// Get the public key for this kid
		publicKey, err := v.getPublicKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Verify issuer
	expectedIssuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", v.region, v.userPoolID)
	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidIssuer, expectedIssuer, claims.Issuer)
	}

	// Verify audience (client ID)
	if len(claims.Audience) == 0 || !v.containsAudience(claims.Audience, v.clientID) {
		return nil, ErrInvalidAudience
	}

	// Verify token use (should be "id" or "access")
	if claims.TokenUse != "id" && claims.TokenUse != "access" {
		return nil, fmt.Errorf("invalid token_use: %s", claims.TokenUse)
	}

	// Parse UUIDs
	sub, err := uuid.Parse(claims.Sub)
	if err != nil {
		return nil, fmt.Errorf("invalid sub UUID: %w", err)
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
		IssuedAt:     claims.IssuedAt.Time,
		ExpiresAt:    claims.ExpiresAt.Time,
	}

	return parsed, nil
}

// FetchJWKS fetches the JWKS from Cognito
func (v *CognitoValidator) FetchJWKS(ctx context.Context) (*JWKS, error) {
	// Check cache first
	v.cacheMu.RLock()
	if v.jwksCache != nil && time.Now().Before(v.jwksCacheExp) {
		defer v.cacheMu.RUnlock()
		return v.jwksCache, nil
	}
	v.cacheMu.RUnlock()

	// Cache miss or expired, fetch from Cognito
	req, err := http.NewRequestWithContext(ctx, "GET", v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status code %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Update cache
	v.cacheMu.Lock()
	v.jwksCache = &jwks
	v.jwksCacheExp = time.Now().Add(v.jwksCacheTTL)
	v.cacheMu.Unlock()

	return &jwks, nil
}

// getPublicKey retrieves the public key for a given kid
func (v *CognitoValidator) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Check key cache first
	v.keyCacheMu.RLock()
	if key, exists := v.keyCache[kid]; exists {
		v.keyCacheMu.RUnlock()
		return key, nil
	}
	v.keyCacheMu.RUnlock()

	// Fetch JWKS
	jwks, err := v.FetchJWKS(ctx)
	if err != nil {
		return nil, err
	}

	// Find the key with matching kid
	var jwk *JWK
	for i := range jwks.Keys {
		if jwks.Keys[i].Kid == kid {
			jwk = &jwks.Keys[i]
			break
		}
	}

	if jwk == nil {
		return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
	}

	// Convert JWK to RSA public key
	publicKey, err := v.jwkToRSAPublicKey(jwk)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JWK to RSA public key: %w", err)
	}

	// Cache the key
	v.keyCacheMu.Lock()
	v.keyCache[kid] = publicKey
	v.keyCacheMu.Unlock()

	return publicKey, nil
}

// jwkToRSAPublicKey converts a JWK to an RSA public key
func (v *CognitoValidator) jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	// Decode the modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode the exponent
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert to big.Int
	n := new(big.Int).SetBytes(nBytes)
	
	// Convert exponent bytes to int
	var e int
	for _, b := range eBytes {
		e = e*256 + int(b)
	}

	// Create RSA public key
	publicKey := &rsa.PublicKey{
		N: n,
		E: e,
	}

	return publicKey, nil
}

// containsAudience checks if the audience list contains the expected client ID
func (v *CognitoValidator) containsAudience(audiences jwt.ClaimStrings, clientID string) bool {
	for _, aud := range audiences {
		if aud == clientID {
			return true
		}
	}
	return false
}

// InvalidateCache invalidates the JWKS cache (useful for testing or forced refresh)
func (v *CognitoValidator) InvalidateCache() {
	v.cacheMu.Lock()
	defer v.cacheMu.Unlock()
	v.jwksCache = nil
	v.jwksCacheExp = time.Time{}
	
	v.keyCacheMu.Lock()
	defer v.keyCacheMu.Unlock()
	v.keyCache = make(map[string]*rsa.PublicKey)
}

// GetCacheStats returns cache statistics
func (v *CognitoValidator) GetCacheStats() map[string]interface{} {
	v.cacheMu.RLock()
	defer v.cacheMu.RUnlock()
	
	v.keyCacheMu.RLock()
	defer v.keyCacheMu.RUnlock()
	
	stats := map[string]interface{}{
		"jwks_cached": v.jwksCache != nil,
		"jwks_expires_at": v.jwksCacheExp,
		"cached_keys_count": len(v.keyCache),
	}
	
	if v.jwksCache != nil {
		stats["jwks_keys_count"] = len(v.jwksCache.Keys)
	}
	
	return stats
}

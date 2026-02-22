package cognito

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to generate RSA key pair
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey, &privateKey.PublicKey
}

// Test helper to create a mock JWKS server
func createMockJWKSServer(t *testing.T, publicKey *rsa.PublicKey, kid string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Convert public key to JWK format
		nBytes := publicKey.N.Bytes()
		eBytes := big.NewInt(int64(publicKey.E)).Bytes()

		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: kid,
					Kty: "RSA",
					Alg: "RS256",
					Use: "sig",
					N:   base64.RawURLEncoding.EncodeToString(nBytes),
					E:   base64.RawURLEncoding.EncodeToString(eBytes),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
}

// Test helper to create a test token
func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, kid, region, userPoolID, clientID string, customClaims map[string]string) string {
	now := time.Now()
	
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID),
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Sub:          uuid.New().String(),
		Email:        "test@example.com",
		EmailVerified: true,
		TokenUse:     "id",
		AuthTime:     now.Unix(),
		CognitoUsername: "testuser",
	}

	// Add custom claims (keys match actions-aws-auth: tenantId, userRole; app_id optional)
	if orgID, ok := customClaims["org_id"]; ok {
		claims.OrgID = orgID // maps to custom:tenantId in JWT
	}
	if appID, ok := customClaims["app_id"]; ok {
		claims.AppID = appID
	}
	if role, ok := customClaims["role"]; ok {
		claims.Role = role // maps to custom:userRole in JWT
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	return tokenString
}

func TestNewCognitoValidator(t *testing.T) {
	config := Config{
		Region:     "us-east-1",
		UserPoolID: "us-east-1_test123",
		ClientID:   "test-client-id",
	}

	validator := NewCognitoValidator(config)

	assert.NotNil(t, validator)
	assert.Equal(t, config.Region, validator.region)
	assert.Equal(t, config.UserPoolID, validator.userPoolID)
	assert.Equal(t, config.ClientID, validator.clientID)
	assert.NotNil(t, validator.httpClient)
	assert.NotNil(t, validator.keyCache)
}

func TestFetchJWKS(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	ctx := context.Background()

	// First fetch - should hit server
	jwks, err := validator.FetchJWKS(ctx)
	require.NoError(t, err)
	assert.NotNil(t, jwks)
	assert.Len(t, jwks.Keys, 1)
	assert.Equal(t, kid, jwks.Keys[0].Kid)

	// Second fetch - should use cache
	jwks2, err := validator.FetchJWKS(ctx)
	require.NoError(t, err)
	assert.Equal(t, jwks, jwks2)

	// Verify cache was used (same pointer)
	assert.True(t, jwks == jwks2)
}

func TestValidateToken_Success(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	orgID := uuid.New().String()
	appID := uuid.New().String()
	
	tokenString := createTestToken(t, privateKey, kid, region, userPoolID, clientID, map[string]string{
		"org_id": orgID,
		"app_id": appID,
		"role":   "admin",
	})

	ctx := context.Background()
	parsedClaims, err := validator.ValidateToken(ctx, tokenString)

	require.NoError(t, err)
	assert.NotNil(t, parsedClaims)
	assert.Equal(t, orgID, parsedClaims.OrgID.String())
	assert.NotNil(t, parsedClaims.AppID)
	assert.Equal(t, appID, parsedClaims.AppID.String())
	assert.Equal(t, "admin", parsedClaims.Role)
	assert.Equal(t, "test@example.com", parsedClaims.Email)
	assert.True(t, parsedClaims.EmailVerified)
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	differentPrivateKey, _ := generateTestKeyPair(t)
	
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	// Sign token with different key
	tokenString := createTestToken(t, differentPrivateKey, kid, region, userPoolID, clientID, map[string]string{
		"org_id": uuid.New().String(),
	})

	ctx := context.Background()
	_, err := validator.ValidateToken(ctx, tokenString)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	// Create expired token
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID),
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
		Sub:          uuid.New().String(),
		Email:        "test@example.com",
		TokenUse:     "id",
		OrgID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = validator.ValidateToken(ctx, tokenString)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestValidateToken_InvalidIssuer(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://evil-issuer.com", // Wrong issuer
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Sub:          uuid.New().String(),
		Email:        "test@example.com",
		TokenUse:     "id",
		OrgID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = validator.ValidateToken(ctx, tokenString)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidIssuer)
}

func TestValidateToken_InvalidAudience(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID),
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{"wrong-client-id"}, // Wrong audience
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Sub:          uuid.New().String(),
		Email:        "test@example.com",
		TokenUse:     "id",
		OrgID:        uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = validator.ValidateToken(ctx, tokenString)

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidAudience)
}

func TestValidateToken_MissingOrgID(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	// Create token without org_id
	tokenString := createTestToken(t, privateKey, kid, region, userPoolID, clientID, map[string]string{
		// No org_id
	})

	ctx := context.Background()
	_, err := validator.ValidateToken(ctx, tokenString)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tenantId")
}

func TestInvalidateCache(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	ctx := context.Background()

	// Fetch JWKS to populate cache
	_, err := validator.FetchJWKS(ctx)
	require.NoError(t, err)

	// Verify cache is populated
	assert.NotNil(t, validator.jwksCache)

	// Invalidate cache
	validator.InvalidateCache()

	// Verify cache is cleared
	assert.Nil(t, validator.jwksCache)
	assert.Equal(t, 0, len(validator.keyCache))
}

func TestGetCacheStats(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	ctx := context.Background()

	// Initially no cache
	stats := validator.GetCacheStats()
	assert.False(t, stats["jwks_cached"].(bool))

	// Fetch JWKS
	_, err := validator.FetchJWKS(ctx)
	require.NoError(t, err)

	// Now cache should be populated
	stats = validator.GetCacheStats()
	assert.True(t, stats["jwks_cached"].(bool))
	assert.Equal(t, 1, stats["jwks_keys_count"].(int))
}

func TestJWKToRSAPublicKey(t *testing.T) {
	_, publicKey := generateTestKeyPair(t)

	// Convert to JWK format
	nBytes := publicKey.N.Bytes()
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()

	jwk := &JWK{
		Kid: "test-kid",
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}

	validator := &CognitoValidator{}
	convertedKey, err := validator.jwkToRSAPublicKey(jwk)

	require.NoError(t, err)
	assert.NotNil(t, convertedKey)
	assert.Equal(t, publicKey.N, convertedKey.N)
	assert.Equal(t, publicKey.E, convertedKey.E)
}

func TestValidateToken_OptionalAppID(t *testing.T) {
	privateKey, publicKey := generateTestKeyPair(t)
	kid := "test-kid-123"
	region := "us-east-1"
	userPoolID := "us-east-1_test123"
	clientID := "test-client-id"

	server := createMockJWKSServer(t, publicKey, kid)
	defer server.Close()

	validator := &CognitoValidator{
		region:       region,
		userPoolID:   userPoolID,
		clientID:     clientID,
		jwksURL:      server.URL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		jwksCacheTTL: 1 * time.Hour,
		keyCache:     make(map[string]*rsa.PublicKey),
	}

	orgID := uuid.New().String()
	
	// Create token without app_id
	tokenString := createTestToken(t, privateKey, kid, region, userPoolID, clientID, map[string]string{
		"org_id": orgID,
		// No app_id
	})

	ctx := context.Background()
	parsedClaims, err := validator.ValidateToken(ctx, tokenString)

	require.NoError(t, err)
	assert.NotNil(t, parsedClaims)
	assert.Equal(t, orgID, parsedClaims.OrgID.String())
	assert.Nil(t, parsedClaims.AppID) // AppID should be nil
}

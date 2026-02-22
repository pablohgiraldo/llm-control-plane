package cognito

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractClaims(t *testing.T) {
	orgID := uuid.New()
	appID := uuid.New()
	sub := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject:   sub.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Sub:             sub.String(),
		Email:           "test@example.com",
		EmailVerified:   true,
		TokenUse:        "id",
		CognitoUsername: "testuser",
		OrgID:           orgID.String(),
		AppID:           appID.String(),
		Role:            "admin",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	parsedClaims, err := ExtractClaims(tokenString)
	require.NoError(t, err)
	assert.NotNil(t, parsedClaims)
	assert.Equal(t, sub, parsedClaims.Sub)
	assert.Equal(t, "test@example.com", parsedClaims.Email)
	assert.Equal(t, orgID, parsedClaims.OrgID)
	assert.NotNil(t, parsedClaims.AppID)
	assert.Equal(t, appID, *parsedClaims.AppID)
	assert.Equal(t, "admin", parsedClaims.Role)
	assert.True(t, parsedClaims.EmailVerified)
	assert.Equal(t, "testuser", parsedClaims.Username)
}

func TestExtractClaims_MissingSub(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
		},
		// Missing Sub
		OrgID: uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractClaims(tokenString)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingClaim)
	assert.Contains(t, err.Error(), "sub")
}

func TestExtractClaims_MissingOrgID(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject: uuid.New().String(),
		},
		Sub: uuid.New().String(),
		// Missing OrgID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractClaims(tokenString)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingClaim)
	assert.Contains(t, err.Error(), "tenantId")
}

func TestExtractClaims_InvalidSubUUID(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject: "not-a-uuid",
		},
		Sub:   "not-a-uuid",
		OrgID: uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractClaims(tokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sub UUID")
}

func TestExtractClaims_InvalidOrgIDUUID(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: "not-a-uuid",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractClaims(tokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid custom:tenantId UUID")
}

func TestExtractClaims_InvalidAppIDUUID(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: uuid.New().String(),
		AppID: "not-a-uuid",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractClaims(tokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "app_id")
}

func TestExtractClaims_OptionalAppID(t *testing.T) {
	orgID := uuid.New()
	sub := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject: sub.String(),
		},
		Sub:   sub.String(),
		OrgID: orgID.String(),
		// AppID is empty (optional)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	parsedClaims, err := ExtractClaims(tokenString)
	require.NoError(t, err)
	assert.NotNil(t, parsedClaims)
	assert.Equal(t, sub, parsedClaims.Sub)
	assert.Equal(t, orgID, parsedClaims.OrgID)
	assert.Nil(t, parsedClaims.AppID)
}

func TestExtractOrgID(t *testing.T) {
	orgID := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: orgID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	extractedOrgID, err := ExtractOrgID(tokenString)
	require.NoError(t, err)
	assert.Equal(t, orgID, extractedOrgID)
}

func TestExtractOrgID_Missing(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
		Sub: uuid.New().String(),
		// Missing OrgID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = ExtractOrgID(tokenString)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingClaim)
}

func TestExtractAppID(t *testing.T) {
	appID := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: uuid.New().String(),
		AppID: appID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	extractedAppID, err := ExtractAppID(tokenString)
	require.NoError(t, err)
	assert.NotNil(t, extractedAppID)
	assert.Equal(t, appID, *extractedAppID)
}

func TestExtractAppID_Optional(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: uuid.New().String(),
		// AppID is empty
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	extractedAppID, err := ExtractAppID(tokenString)
	require.NoError(t, err)
	assert.Nil(t, extractedAppID)
}

func TestExtractRole(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
		Sub:   uuid.New().String(),
		OrgID: uuid.New().String(),
		Role:  "developer",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	role, err := ExtractRole(tokenString)
	require.NoError(t, err)
	assert.Equal(t, "developer", role)
}

func TestValidateCustomClaims(t *testing.T) {
	tests := []struct {
		name        string
		claims      *Claims
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid claims",
			claims: &Claims{
				OrgID: uuid.New().String(),
				AppID: uuid.New().String(),
				Role:  "admin",
			},
			expectError: false,
		},
		{
			name: "valid claims without app_id",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "developer",
			},
			expectError: false,
		},
		{
			name:        "missing org_id",
			claims:      &Claims{},
			expectError: true,
			errorMsg:    "tenantId",
		},
		{
			name: "invalid org_id format",
			claims: &Claims{
				OrgID: "not-a-uuid",
			},
			expectError: true,
			errorMsg:    "tenantId",
		},
		{
			name: "invalid app_id format",
			claims: &Claims{
				OrgID: uuid.New().String(),
				AppID: "not-a-uuid",
			},
			expectError: true,
			errorMsg:    "app_id",
		},
		{
			name: "invalid role",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "superadmin", // Not in valid roles list
			},
			expectError: true,
			errorMsg:    "userRole",
		},
		{
			name: "valid role - admin",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "admin",
			},
			expectError: false,
		},
		{
			name: "valid role - developer",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "developer",
			},
			expectError: false,
		},
		{
			name: "valid role - user",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "user",
			},
			expectError: false,
		},
		{
			name: "valid role - viewer",
			claims: &Claims{
				OrgID: uuid.New().String(),
				Role:  "viewer",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomClaims(tt.claims)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParsedClaims_ToUserContext(t *testing.T) {
	sub := uuid.New()
	orgID := uuid.New()
	appID := uuid.New()

	parsedClaims := &ParsedClaims{
		Sub:           sub,
		Email:         "test@example.com",
		OrgID:         orgID,
		AppID:         &appID,
		Role:          "admin",
		EmailVerified: true,
		Username:      "testuser",
	}

	userCtx := parsedClaims.ToUserContext()

	assert.NotNil(t, userCtx)
	assert.Equal(t, sub, userCtx.UserID)
	assert.Equal(t, "test@example.com", userCtx.Email)
	assert.Equal(t, orgID, userCtx.OrgID)
	assert.NotNil(t, userCtx.AppID)
	assert.Equal(t, appID, *userCtx.AppID)
	assert.Equal(t, "admin", userCtx.Role)
	assert.True(t, userCtx.EmailVerified)
	assert.Equal(t, "testuser", userCtx.Username)
}

func TestUserContext_IsAdmin(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"developer", false},
		{"user", false},
		{"viewer", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("role_%s", tt.role), func(t *testing.T) {
			userCtx := &UserContext{Role: tt.role}
			assert.Equal(t, tt.expected, userCtx.IsAdmin())
		})
	}
}

func TestUserContext_IsDeveloper(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"developer", true},
		{"user", false},
		{"viewer", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("role_%s", tt.role), func(t *testing.T) {
			userCtx := &UserContext{Role: tt.role}
			assert.Equal(t, tt.expected, userCtx.IsDeveloper())
		})
	}
}

func TestUserContext_HasRole(t *testing.T) {
	userCtx := &UserContext{Role: "developer"}

	assert.True(t, userCtx.HasRole("developer"))
	assert.False(t, userCtx.HasRole("admin"))
	assert.False(t, userCtx.HasRole("user"))
}

func TestUserContext_HasAnyRole(t *testing.T) {
	userCtx := &UserContext{Role: "developer"}

	assert.True(t, userCtx.HasAnyRole("admin", "developer", "user"))
	assert.True(t, userCtx.HasAnyRole("developer"))
	assert.False(t, userCtx.HasAnyRole("admin", "user"))
	assert.False(t, userCtx.HasAnyRole("viewer"))
}

func TestExtractClaimsFromValidatedToken(t *testing.T) {
	orgID := uuid.New()
	appID := uuid.New()
	sub := uuid.New()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test",
			Subject:   sub.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Sub:             sub.String(),
		Email:           "test@example.com",
		EmailVerified:   true,
		TokenUse:        "id",
		CognitoUsername: "testuser",
		OrgID:           orgID.String(),
		AppID:           appID.String(),
		Role:            "developer",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)

	parsedClaims, err := ExtractClaimsFromValidatedToken(token)
	require.NoError(t, err)
	assert.NotNil(t, parsedClaims)
	assert.Equal(t, sub, parsedClaims.Sub)
	assert.Equal(t, orgID, parsedClaims.OrgID)
	assert.Equal(t, appID, *parsedClaims.AppID)
	assert.Equal(t, "developer", parsedClaims.Role)
}

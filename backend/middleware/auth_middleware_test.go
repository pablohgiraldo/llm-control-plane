package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/upb/llm-control-plane/backend/cognito"
	"go.uber.org/zap"
)

// MockTokenValidator is a mock implementation of TokenValidator
type MockTokenValidator struct {
	mock.Mock
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Claims), args.Error(1)
}

func TestRequireAuth(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("valid JWT in Authorization header allows request", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:   "user-123",
			Email: "user@example.com",
			OrgID: uuid.New().String(),
			AppID: uuid.New().String(),
		}
		
		mockValidator.On("ValidateToken", mock.Anything, "valid-token").Return(claims, nil)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify claims are in context
			ctx := r.Context()
			extractedClaims := GetClaimsFromContext(ctx)
			assert.NotNil(t, extractedClaims)
			assert.Equal(t, claims.Sub, extractedClaims.Sub)
			assert.Equal(t, claims.Email, extractedClaims.Email)
			
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		mockValidator.AssertExpectations(t)
	})
	
	t.Run("valid JWT in cookie allows request", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:   "user-456",
			Email: "cookie-user@example.com",
			OrgID: uuid.New().String(),
			AppID: uuid.New().String(),
		}
		
		mockValidator.On("ValidateToken", mock.Anything, "cookie-token-value").Return(claims, nil)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			extractedClaims := GetClaimsFromContext(ctx)
			assert.NotNil(t, extractedClaims)
			assert.Equal(t, claims.Sub, extractedClaims.Sub)
			assert.Equal(t, claims.Email, extractedClaims.Email)
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "cookie-token-value"})
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		mockValidator.AssertExpectations(t)
	})
	
	t.Run("missing token returns 401", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockValidator.AssertNotCalled(t, "ValidateToken")
	})
	
	t.Run("invalid authorization header format returns 401", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockValidator.AssertNotCalled(t, "ValidateToken")
	})
	
	t.Run("invalid token returns 401", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		mockValidator.On("ValidateToken", mock.Anything, "invalid-token").
			Return(nil, errors.New("token validation failed"))
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockValidator.AssertExpectations(t)
	})
	
	t.Run("expired token returns 401", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		mockValidator.On("ValidateToken", mock.Anything, "expired-token").
			Return(nil, cognito.ErrTokenExpired)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockValidator.AssertExpectations(t)
	})
	
	t.Run("claims added to context correctly", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:    "claims-test-user",
			Email:  "claims@example.com",
			OrgID:  uuid.New().String(),
			AppID:  uuid.New().String(),
			Groups: []string{"admin"},
		}
		
		mockValidator.On("ValidateToken", mock.Anything, "claims-token").Return(claims, nil)
		
		handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			extractedClaims := GetClaimsFromContext(ctx)
			assert.NotNil(t, extractedClaims)
			assert.Equal(t, claims.Sub, extractedClaims.Sub)
			assert.Equal(t, claims.Email, extractedClaims.Email)
			assert.Equal(t, claims.OrgID, extractedClaims.OrgID)
			assert.Equal(t, claims.AppID, extractedClaims.AppID)
			assert.Equal(t, claims.Groups, extractedClaims.Groups)
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer claims-token")
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		mockValidator.AssertExpectations(t)
	})
}

func TestExtractTenant(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("successful tenant extraction", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		orgID := uuid.New()
		appID := uuid.New()
		userID := uuid.New()
		
		claims := &Claims{
			Sub:    "user-123",
			Email:  "user@example.com",
			OrgID:  orgID.String(),
			AppID:  appID.String(),
			UserID: userID.String(),
		}
		
		handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			
			// Verify tenant information is in context
			extractedOrgID := GetOrgIDFromContext(ctx)
			assert.Equal(t, orgID, extractedOrgID)
			
			extractedAppID := GetAppIDFromContext(ctx)
			assert.Equal(t, appID, extractedAppID)
			
			extractedUserID := GetUserIDFromContext(ctx)
			assert.NotNil(t, extractedUserID)
			assert.Equal(t, userID, *extractedUserID)
			
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
	
	t.Run("missing claims in context", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	
	t.Run("invalid org_id in claims", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:   "user-123",
			Email: "user@example.com",
			OrgID: "invalid-uuid",
			AppID: uuid.New().String(),
		}
		
		handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
	
	t.Run("invalid app_id in claims", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:   "user-123",
			Email: "user@example.com",
			OrgID: uuid.New().String(),
			AppID: "invalid-uuid",
		}
		
		handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
	
	t.Run("invalid user_id in claims - continues anyway", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		orgID := uuid.New()
		appID := uuid.New()
		
		claims := &Claims{
			Sub:    "user-123",
			Email:  "user@example.com",
			OrgID:  orgID.String(),
			AppID:  appID.String(),
			UserID: "invalid-uuid",
		}
		
		handler := middleware.ExtractTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			
			// Verify tenant information is in context
			extractedOrgID := GetOrgIDFromContext(ctx)
			assert.Equal(t, orgID, extractedOrgID)
			
			extractedAppID := GetAppIDFromContext(ctx)
			assert.Equal(t, appID, extractedAppID)
			
			// User ID should be nil due to invalid format
			extractedUserID := GetUserIDFromContext(ctx)
			assert.Nil(t, extractedUserID)
			
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRequireRole(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("user has required role", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:    "user-123",
			Email:  "user@example.com",
			Groups: []string{"admin", "user"},
		}
		
		handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
	
	t.Run("user does not have required role", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		claims := &Claims{
			Sub:    "user-123",
			Email:  "user@example.com",
			Groups: []string{"user"},
		}
		
		handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := WithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
	
	t.Run("missing claims in context", func(t *testing.T) {
		mockValidator := new(MockTokenValidator)
		middleware := NewAuthMiddleware(mockValidator, logger)
		
		handler := middleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}))
		
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		cookieValue   string
		expectedToken string
	}{
		{
			name:          "valid Bearer token in header",
			authHeader:    "Bearer valid-token-123",
			expectedToken: "valid-token-123",
		},
		{
			name:          "Bearer with lowercase",
			authHeader:    "bearer valid-token-123",
			expectedToken: "valid-token-123",
		},
		{
			name:          "token from auth_token cookie when no header",
			cookieValue:   "cookie-token-value",
			expectedToken: "cookie-token-value",
		},
		{
			name:          "Authorization header takes precedence over cookie",
			authHeader:    "Bearer header-token",
			cookieValue:   "cookie-token",
			expectedToken: "header-token",
		},
		{
			name:          "missing both returns empty",
			expectedToken: "",
		},
		{
			name:          "invalid header format - no space",
			authHeader:    "Bearertoken",
			cookieValue:   "cookie-token",
			expectedToken: "cookie-token",
		},
		{
			name:          "invalid format - wrong prefix falls back to cookie",
			authHeader:    "Basic token",
			cookieValue:   "cookie-token",
			expectedToken: "cookie-token",
		},
		{
			name:          "empty Bearer token falls back to cookie",
			authHeader:    "Bearer ",
			cookieValue:   "cookie-token",
			expectedToken: "cookie-token",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{Name: "auth_token", Value: tt.cookieValue})
			}
			
			token := extractToken(req)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

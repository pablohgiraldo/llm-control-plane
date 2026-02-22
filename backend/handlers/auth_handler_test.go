package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/auth"
	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/cognito"
	"go.uber.org/zap"
)

// MockTokenExchanger mocks the OAuth2 token exchange
type MockTokenExchanger struct {
	mock.Mock
}

func (m *MockTokenExchanger) ExchangeCode(ctx context.Context, code, redirectURI, state string) (idToken string, err error) {
	args := m.Called(ctx, code, redirectURI, state)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.String(0), args.Error(1)
}

// MockTokenValidator mocks JWT validation
type MockTokenValidator struct {
	mock.Mock
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, token string) (*cognito.ParsedClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*cognito.ParsedClaims), args.Error(1)
}

func TestHandleLogin(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Cognito: config.CognitoConfig{
			Domain:      "https://test.auth.us-east-1.amazoncognito.com",
			ClientID:    "test-client-id",
			RedirectURI: "http://localhost:8080/auth/callback",
		},
	}

	t.Run("redirects to Cognito with correct URL format", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.HandleLogin(rec, req)

		require.Equal(t, http.StatusFound, rec.Code)
		loc := rec.Header().Get("Location")
		require.NotEmpty(t, loc)

		parsed, err := url.Parse(loc)
		require.NoError(t, err)

		assert.Contains(t, parsed.Path, "/oauth2/authorize")
		assert.Equal(t, "test.auth.us-east-1.amazoncognito.com", parsed.Host)
		assert.Equal(t, "code", parsed.Query().Get("response_type"))
		assert.Equal(t, "test-client-id", parsed.Query().Get("client_id"))
		assert.Equal(t, "http://localhost:8080/auth/callback", parsed.Query().Get("redirect_uri"))
		assert.NotEmpty(t, parsed.Query().Get("state"))
		assert.Contains(t, parsed.Query().Get("scope"), "openid")
	})

	t.Run("generates unique state parameter for CSRF protection", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		states := make(map[string]bool)
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			rec := httptest.NewRecorder()
			handler.HandleLogin(rec, req)

			loc := rec.Header().Get("Location")
			parsed, _ := url.Parse(loc)
			state := parsed.Query().Get("state")
			assert.False(t, states[state], "state should be unique")
			states[state] = true
		}
	})

	t.Run("sets state cookie for callback verification", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
		rec := httptest.NewRecorder()
		handler.HandleLogin(rec, req)

		cookies := rec.Result().Cookies()
		var stateCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == auth.StateCookieName {
				stateCookie = c
				break
			}
		}
		require.NotNil(t, stateCookie)
		assert.NotEmpty(t, stateCookie.Value)
		assert.True(t, stateCookie.HttpOnly)
		assert.True(t, stateCookie.Secure || !strings.HasPrefix(cfg.Cognito.RedirectURI, "https"))
	})
}

func TestHandleCallback(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Cognito: config.CognitoConfig{
			Domain:      "https://test.auth.us-east-1.amazoncognito.com",
			ClientID:    "test-client-id",
			RedirectURI: "http://localhost:8080/auth/callback",
		},
	}

	validIDToken := "valid.id.token"

	t.Run("exchanges code and sets session cookie on success", func(t *testing.T) {
		mockExchanger := new(MockTokenExchanger)
		mockValidator := new(MockTokenValidator)

		mockExchanger.On("ExchangeCode", mock.Anything, "auth-code", "http://localhost:8080/auth/callback", "state-123").
			Return(validIDToken, nil)
		mockValidator.On("ValidateToken", mock.Anything, validIDToken).
			Return(&cognito.ParsedClaims{}, nil)

		handler := auth.NewHandler(cfg, mockExchanger, mockValidator, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code&state=state-123", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "state-123"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		require.Equal(t, http.StatusFound, rec.Code)
		loc := rec.Header().Get("Location")
		assert.Equal(t, "/", loc, "default redirect when FrontEndURL not set")

		cookies := rec.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == auth.SessionCookieName {
				sessionCookie = c
				break
			}
		}
		require.NotNil(t, sessionCookie)
		assert.Equal(t, validIDToken, sessionCookie.Value)
		assert.True(t, sessionCookie.HttpOnly)
		assert.True(t, sessionCookie.Secure || strings.Contains(cfg.Cognito.RedirectURI, "localhost"))
		assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite)

		mockExchanger.AssertExpectations(t)
		mockValidator.AssertExpectations(t)
	})

	t.Run("redirects to FrontEndURL when configured", func(t *testing.T) {
		cfgWithFrontend := &config.Config{
			Cognito: config.CognitoConfig{
				Domain:      "https://test.auth.us-east-1.amazoncognito.com",
				ClientID:    "test-client-id",
				RedirectURI: "http://localhost:8080/auth/callback",
				FrontEndURL: "http://localhost:5173",
			},
		}
		mockExchanger := new(MockTokenExchanger)
		mockValidator := new(MockTokenValidator)

		mockExchanger.On("ExchangeCode", mock.Anything, "auth-code", "http://localhost:8080/auth/callback", "state-123").
			Return(validIDToken, nil)
		mockValidator.On("ValidateToken", mock.Anything, validIDToken).
			Return(&cognito.ParsedClaims{}, nil)

		handler := auth.NewHandler(cfgWithFrontend, mockExchanger, mockValidator, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code&state=state-123", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "state-123"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		require.Equal(t, http.StatusFound, rec.Code)
		loc := rec.Header().Get("Location")
		assert.Equal(t, "http://localhost:5173", loc)

		cookies := rec.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == auth.SessionCookieName {
				sessionCookie = c
				break
			}
		}
		require.NotNil(t, sessionCookie)
		assert.Equal(t, validIDToken, sessionCookie.Value)
		assert.True(t, sessionCookie.HttpOnly)
		assert.True(t, sessionCookie.Secure || strings.Contains(cfgWithFrontend.Cognito.RedirectURI, "localhost"))
		assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite)

		mockExchanger.AssertExpectations(t)
		mockValidator.AssertExpectations(t)
	})

	t.Run("returns bad request when code is missing", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=state-123", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "state-123"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns bad request when state is missing", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code", nil)
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns bad request when state does not match cookie", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code&state=wrong-state", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "correct-state"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns unauthorized when token exchange fails", func(t *testing.T) {
		mockExchanger := new(MockTokenExchanger)
		mockExchanger.On("ExchangeCode", mock.Anything, "bad-code", "http://localhost:8080/auth/callback", "state-123").
			Return("", assert.AnError)

		handler := auth.NewHandler(cfg, mockExchanger, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=bad-code&state=state-123", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "state-123"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		mockExchanger.AssertExpectations(t)
	})

	t.Run("returns unauthorized when JWT validation fails", func(t *testing.T) {
		mockExchanger := new(MockTokenExchanger)
		mockValidator := new(MockTokenValidator)

		mockExchanger.On("ExchangeCode", mock.Anything, "auth-code", "http://localhost:8080/auth/callback", "state-123").
			Return("invalid.token", nil)
		mockValidator.On("ValidateToken", mock.Anything, "invalid.token").
			Return(nil, cognito.ErrInvalidToken)

		handler := auth.NewHandler(cfg, mockExchanger, mockValidator, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code&state=state-123", nil)
		req.AddCookie(&http.Cookie{Name: auth.StateCookieName, Value: "state-123"})
		rec := httptest.NewRecorder()

		handler.HandleCallback(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		mockExchanger.AssertExpectations(t)
		mockValidator.AssertExpectations(t)
	})
}

func TestHandleLogout(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Cognito: config.CognitoConfig{
			Domain:      "https://test.auth.us-east-1.amazoncognito.com",
			ClientID:    "test-client-id",
			RedirectURI: "http://localhost:8080/auth/callback",
		},
	}

	t.Run("clears session cookie and redirects to Cognito logout", func(t *testing.T) {
		handler := auth.NewHandler(cfg, nil, nil, logger)

		req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
		rec := httptest.NewRecorder()

		handler.HandleLogout(rec, req)

		require.Equal(t, http.StatusFound, rec.Code)
		loc := rec.Header().Get("Location")
		require.NotEmpty(t, loc)

		parsed, err := url.Parse(loc)
		require.NoError(t, err)
		assert.Contains(t, parsed.Path, "/logout")
		assert.Equal(t, "test.auth.us-east-1.amazoncognito.com", parsed.Host)
		assert.Equal(t, "test-client-id", parsed.Query().Get("client_id"))
		assert.NotEmpty(t, parsed.Query().Get("logout_uri"))

		cookies := rec.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == auth.SessionCookieName {
				sessionCookie = c
				break
			}
		}
		require.NotNil(t, sessionCookie, "should set cookie to clear session")
		assert.Empty(t, sessionCookie.Value)
		assert.True(t, sessionCookie.MaxAge < 0, "cookie should be expired to clear it")
	})
}


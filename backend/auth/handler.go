package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/cognito"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

const (
	// StateCookieName is the cookie name for OAuth state (CSRF)
	StateCookieName = "oauth_state"
	// SessionCookieName is the cookie name for the session token
	SessionCookieName = "session"
	stateCookieMaxAge = 600
	sessionCookieMaxAge = 86400 * 7 // 7 days
)

// TokenExchanger exchanges OAuth2 authorization codes for tokens via the OAuth2 token endpoint.
type TokenExchanger interface {
	ExchangeCode(ctx context.Context, code, redirectURI, state string) (idToken string, err error)
}

// TokenValidator validates JWT tokens and returns parsed claims.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*cognito.ParsedClaims, error)
}

// Handler handles OAuth2 authentication flows (login, callback, logout).
type Handler struct {
	cfg       *config.Config
	exchanger TokenExchanger
	validator TokenValidator
	logger    *zap.Logger
}

// NewHandler creates a new auth handler with the given config, token exchanger, and validator.
func NewHandler(cfg *config.Config, exchanger TokenExchanger, validator TokenValidator, logger *zap.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		exchanger: exchanger,
		validator: validator,
		logger:    logger,
	}
}

// HandleLogin redirects to Cognito hosted UI for OAuth2 authorization
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Cognito.Domain == "" || h.cfg.Cognito.ClientID == "" {
		h.logger.Error("cognito not configured")
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
		return
	}

	state, err := generateSecureState()
	if err != nil {
		h.logger.Error("failed to generate state", zap.Error(err))
		_ = utils.WriteInternalServerError(w, "Failed to initiate login")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   stateCookieMaxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.cfg.Cognito.RedirectURI, "https"),
		SameSite: http.SameSiteStrictMode,
	})

	authURL := buildAuthURL(h.cfg.Cognito.Domain, h.cfg.Cognito.ClientID, h.cfg.Cognito.RedirectURI, state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback exchanges the authorization code for tokens, validates the JWT, and sets the session cookie
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		_ = utils.WriteBadRequest(w, "Missing authorization code", nil)
		return
	}
	if state == "" {
		_ = utils.WriteBadRequest(w, "Missing state parameter", nil)
		return
	}

	stateCookie, err := r.Cookie(StateCookieName)
	if err != nil || stateCookie.Value != state {
		_ = utils.WriteBadRequest(w, "Invalid or expired state", nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     StateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.cfg.Cognito.RedirectURI, "https"),
		SameSite: http.SameSiteStrictMode,
	})

	if h.exchanger == nil {
		h.logger.Error("token exchanger not configured")
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
		return
	}

	idToken, err := h.exchanger.ExchangeCode(r.Context(), code, h.cfg.Cognito.RedirectURI, state)
	if err != nil {
		h.logger.Warn("token exchange failed", zap.Error(err))
		_ = utils.WriteUnauthorized(w, "Authentication failed")
		return
	}

	if h.validator == nil {
		h.logger.Error("token validator not configured")
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
		return
	}

	_, err = h.validator.ValidateToken(r.Context(), idToken)
	if err != nil {
		h.logger.Warn("token validation failed", zap.Error(err))
		_ = utils.WriteUnauthorized(w, "Invalid token")
		return
	}

	secure := strings.HasPrefix(h.cfg.Cognito.RedirectURI, "https")
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    idToken,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})

	redirectURL := h.cfg.Cognito.FrontEndURL
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleLogout clears the session cookie and redirects to Cognito logout
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   strings.HasPrefix(h.cfg.Cognito.RedirectURI, "https"),
		SameSite: http.SameSiteStrictMode,
	})

	logoutURL := buildLogoutURL(h.cfg.Cognito.Domain, h.cfg.Cognito.ClientID, h.cfg.Cognito.RedirectURI)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

func buildAuthURL(domain, clientID, redirectURI, state string) string {
	base := strings.TrimSuffix(domain, "/") + "/oauth2/authorize"
	params := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"state":         {state},
		"scope":         {"openid email profile"},
	}
	return base + "?" + params.Encode()
}

func buildLogoutURL(domain, clientID, redirectURI string) string {
	parsed, err := url.Parse(redirectURI)
	logoutURI := redirectURI
	if err == nil {
		logoutURI = parsed.Scheme + "://" + parsed.Host
	}
	base := strings.TrimSuffix(domain, "/") + "/logout"
	params := url.Values{
		"client_id":  {clientID},
		"logout_uri": {logoutURI},
	}
	return base + "?" + params.Encode()
}

func generateSecureState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

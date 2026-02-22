package handlers

import (
	"net/http"

	"github.com/upb/llm-control-plane/backend/auth"
	"github.com/upb/llm-control-plane/backend/utils"
)

// AuthDeps provides auth handler for route wiring
type AuthDeps interface {
	AuthHandler() *auth.Handler
}

// AuthLoginHandler returns an http.HandlerFunc for the login endpoint
func AuthLoginHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h := deps.AuthHandler(); h != nil {
			h.HandleLogin(w, r)
			return
		}
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
	}
}

// AuthCallbackHandler returns an http.HandlerFunc for the OAuth callback endpoint
func AuthCallbackHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h := deps.AuthHandler(); h != nil {
			h.HandleCallback(w, r)
			return
		}
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
	}
}

// AuthLogoutHandler returns an http.HandlerFunc for the logout endpoint
func AuthLogoutHandler(deps AuthDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h := deps.AuthHandler(); h != nil {
			h.HandleLogout(w, r)
			return
		}
		_ = utils.WriteInternalServerError(w, "Authentication not configured")
	}
}

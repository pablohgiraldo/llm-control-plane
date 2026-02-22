package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/app"
	"github.com/upb/llm-control-plane/backend/middleware"
	"go.uber.org/zap"
)

func TestGetCurrentUserHandler(t *testing.T) {
	logger := zap.NewNop()
	deps := &app.Dependencies{Logger: logger}

	t.Run("returns 200 with user info when authenticated", func(t *testing.T) {
		claims := &middleware.Claims{
			Sub:           "user-123",
			Email:         "user@example.com",
			EmailVerified: true,
			OrgID:         uuid.New().String(),
			AppID:         uuid.New().String(),
			UserID:        "user-123",
			Groups:        []string{"user"},
		}

		handler := GetCurrentUserHandler(deps)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req = req.WithContext(middleware.WithClaims(req.Context(), claims))
		rec := httptest.NewRecorder()

		handler(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var body struct {
			Data struct {
				Sub           string   `json:"sub"`
				Email         string   `json:"email"`
				EmailVerified bool     `json:"email_verified"`
				Groups        []string `json:"groups"`
			} `json:"data"`
		}
		err := json.NewDecoder(rec.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "user-123", body.Data.Sub)
		assert.Equal(t, "user@example.com", body.Data.Email)
		assert.True(t, body.Data.EmailVerified)
		assert.Equal(t, []string{"user"}, body.Data.Groups)
	})

	t.Run("returns 401 when claims missing in context", func(t *testing.T) {
		// Defensive: if claims are nil (should not happen after RequireAuth), return 401
		handler := GetCurrentUserHandler(deps)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		// No claims in context - req.Context() has no claims
		rec := httptest.NewRecorder()

		handler(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

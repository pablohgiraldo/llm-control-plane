package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/services"
	"github.com/upb/llm-control-plane/backend/utils"
	"go.uber.org/zap"
)

func TestHandleServiceError(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "not found error",
			err:            services.ErrPolicyNotFound,
			expectedStatus: http.StatusNotFound,
			expectedError:  "not_found",
		},
		{
			name:           "validation error",
			err:            services.ErrInvalidInput,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "bad_request",
		},
		{
			name:           "unauthorized error",
			err:            services.ErrUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
		},
		{
			name:           "forbidden error",
			err:            services.ErrForbidden,
			expectedStatus: http.StatusForbidden,
			expectedError:  "forbidden",
		},
		{
			name:           "rate limit error",
			err:            services.ErrRateLimitExceeded,
			expectedStatus: http.StatusTooManyRequests,
			expectedError:  "rate_limit_exceeded",
		},
		{
			name:           "budget error",
			err:            services.ErrBudgetExceeded,
			expectedStatus: http.StatusTooManyRequests,
			expectedError:  "rate_limit_exceeded",
		},
		{
			name:           "conflict error",
			err:            services.ErrDuplicateSlug,
			expectedStatus: http.StatusConflict,
			expectedError:  "conflict",
		},
		{
			name:           "policy violation error",
			err:            services.ErrPolicyViolation,
			expectedStatus: http.StatusForbidden,
			expectedError:  "forbidden",
		},
		{
			name:           "external provider error",
			err:            services.ErrProviderUnavailable,
			expectedStatus: http.StatusBadGateway,
			expectedError:  "bad_gateway",
		},
		{
			name:           "internal error",
			err:            services.ErrInternal,
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal_error",
		},
		{
			name:           "unknown error",
			err:            errors.New("some unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			HandleServiceError(w, tt.err, logger)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response utils.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedError, response.Error)
			assert.NotEmpty(t, response.Message)
		})
	}
}

func TestHandleServiceErrorWithDetails(t *testing.T) {
	logger := zap.NewNop()

	// Create error with details
	err := services.ErrRateLimitExceeded.WithDetail("limit", 100).WithDetail("window", "minute")

	w := httptest.NewRecorder()
	HandleServiceError(w, err, logger)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var response utils.ErrorResponse
	err2 := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err2)

	assert.Equal(t, "rate_limit_exceeded", response.Error)
	assert.NotNil(t, response.Details)
	assert.Equal(t, float64(100), response.Details["limit"])
	assert.Equal(t, "minute", response.Details["window"])
}

func TestHandleServiceErrorNil(t *testing.T) {
	logger := zap.NewNop()
	w := httptest.NewRecorder()

	HandleServiceError(w, nil, logger)

	// Should not write anything
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestHandleValidationError(t *testing.T) {
	logger := zap.NewNop()

	t.Run("custom validation error", func(t *testing.T) {
		fields := map[string]string{
			"email": "email is required",
			"name":  "name must be at least 3 characters",
		}
		err := &utils.ValidationError{
			Message: "Validation failed",
			Fields:  fields,
		}

		w := httptest.NewRecorder()
		HandleValidationError(w, err, logger)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response utils.ErrorResponse
		err2 := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err2)

		assert.Equal(t, "bad_request", response.Error)
		assert.Equal(t, "Validation failed", response.Message)
		assert.NotNil(t, response.Details)
		assert.Equal(t, "email is required", response.Details["email"])
		assert.Equal(t, "name must be at least 3 characters", response.Details["name"])
	})

	t.Run("generic error", func(t *testing.T) {
		err := errors.New("generic validation error")

		w := httptest.NewRecorder()
		HandleValidationError(w, err, logger)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response utils.ErrorResponse
		err2 := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err2)

		assert.Equal(t, "bad_request", response.Error)
		assert.Equal(t, "generic validation error", response.Message)
	})
}

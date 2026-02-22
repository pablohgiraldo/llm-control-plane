package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "test"}
		
		err := WriteJSON(w, http.StatusOK, data)
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		
		var response map[string]string
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "test", response["message"])
	})
	
	t.Run("nil data", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteJSON(w, http.StatusNoContent, nil)
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})
}

func TestWriteOK(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"result": "success"}
	
	err := WriteOK(w, data)
	require.NoError(t, err)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response SuccessResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	dataMap := response.Data.(map[string]interface{})
	assert.Equal(t, "success", dataMap["result"])
}

func TestWriteCreated(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"id": "123"}
	
	err := WriteCreated(w, data)
	require.NoError(t, err)
	
	assert.Equal(t, http.StatusCreated, w.Code)
	
	var response SuccessResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	dataMap := response.Data.(map[string]interface{})
	assert.Equal(t, "123", dataMap["id"])
}

func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	
	WriteNoContent(w)
	
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestWriteBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	details := map[string]interface{}{"email": "invalid format"}
	
	err := WriteBadRequest(w, "Validation failed", details)
	require.NoError(t, err)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	assert.Equal(t, "bad_request", response.Error)
	assert.Equal(t, "Validation failed", response.Message)
	assert.Equal(t, "invalid format", response.Details["email"])
}

func TestWriteUnauthorized(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteUnauthorized(w, "Invalid token")
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "unauthorized", response.Error)
		assert.Equal(t, "Invalid token", response.Message)
	})
	
	t.Run("with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteUnauthorized(w, "")
		require.NoError(t, err)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "Authentication required", response.Message)
	})
}

func TestWriteForbidden(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteForbidden(w, "Insufficient permissions")
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusForbidden, w.Code)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "forbidden", response.Error)
		assert.Equal(t, "Insufficient permissions", response.Message)
	})
	
	t.Run("with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteForbidden(w, "")
		require.NoError(t, err)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "Access forbidden", response.Message)
	})
}

func TestWriteNotFound(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteNotFound(w, "User not found")
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "not_found", response.Error)
		assert.Equal(t, "User not found", response.Message)
	})
	
	t.Run("with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteNotFound(w, "")
		require.NoError(t, err)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "Resource not found", response.Message)
	})
}

func TestWriteConflict(t *testing.T) {
	w := httptest.NewRecorder()
	details := map[string]interface{}{"field": "email", "conflict": "already exists"}
	
	err := WriteConflict(w, "Email already exists", details)
	require.NoError(t, err)
	
	assert.Equal(t, http.StatusConflict, w.Code)
	
	var response ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	
	assert.Equal(t, "conflict", response.Error)
	assert.Equal(t, "Email already exists", response.Message)
	assert.Equal(t, "already exists", response.Details["conflict"])
}

func TestWriteTooManyRequests(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		w := httptest.NewRecorder()
		details := map[string]interface{}{"limit": 100, "window": "minute"}
		
		err := WriteTooManyRequests(w, "Exceeded rate limit", details)
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "rate_limit_exceeded", response.Error)
		assert.Equal(t, "Exceeded rate limit", response.Message)
		assert.Equal(t, float64(100), response.Details["limit"])
	})
	
	t.Run("with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteTooManyRequests(w, "", nil)
		require.NoError(t, err)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "Rate limit exceeded", response.Message)
	})
}

func TestWriteInternalServerError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteInternalServerError(w, "Database connection failed")
		require.NoError(t, err)
		
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "internal_error", response.Error)
		assert.Equal(t, "Database connection failed", response.Message)
	})
	
	t.Run("with empty message", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		err := WriteInternalServerError(w, "")
		require.NoError(t, err)
		
		var response ErrorResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		assert.Equal(t, "Internal server error", response.Message)
	})
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name               string
		status             int
		message            string
		expectedErrorType  string
	}{
		{
			name:              "bad request",
			status:            http.StatusBadRequest,
			message:           "Invalid input",
			expectedErrorType: "bad_request",
		},
		{
			name:              "unauthorized",
			status:            http.StatusUnauthorized,
			message:           "Not authenticated",
			expectedErrorType: "unauthorized",
		},
		{
			name:              "forbidden",
			status:            http.StatusForbidden,
			message:           "No access",
			expectedErrorType: "forbidden",
		},
		{
			name:              "not found",
			status:            http.StatusNotFound,
			message:           "Not found",
			expectedErrorType: "not_found",
		},
		{
			name:              "conflict",
			status:            http.StatusConflict,
			message:           "Conflict",
			expectedErrorType: "conflict",
		},
		{
			name:              "rate limit",
			status:            http.StatusTooManyRequests,
			message:           "Too many requests",
			expectedErrorType: "rate_limit_exceeded",
		},
		{
			name:              "unknown status defaults to internal error",
			status:            http.StatusTeapot,
			message:           "I'm a teapot",
			expectedErrorType: "internal_error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			
			err := WriteError(w, tt.status, tt.message, nil)
			require.NoError(t, err)
			
			assert.Equal(t, tt.status, w.Code)
			
			var response ErrorResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectedErrorType, response.Error)
			assert.Equal(t, tt.message, response.Message)
		})
	}
}

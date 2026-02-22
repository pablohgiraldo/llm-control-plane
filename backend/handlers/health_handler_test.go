package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleHealth(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("always returns healthy", func(t *testing.T) {
		handler := NewHealthHandler(nil, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		
		handler.HandleHealth(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "healthy", data["status"])
		assert.NotEmpty(t, data["timestamp"])
	})
}

func TestHandleReadiness(t *testing.T) {
	logger := zap.NewNop()
	
	t.Run("healthy when database is available", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		
		// Expect ping
		mock.ExpectPing()
		
		// Expect SELECT 1 query
		mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		
		handler := NewHealthHandler(db, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		
		handler.HandleReadiness(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "healthy", data["status"])
		
		checks := data["checks"].(map[string]interface{})
		assert.Equal(t, "healthy", checks["database"])
		
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	
	t.Run("unhealthy when database ping fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		
		// Expect ping to fail
		mock.ExpectPing().WillReturnError(sql.ErrConnDone)
		
		handler := NewHealthHandler(db, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		
		handler.HandleReadiness(w, req)
		
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		
		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "unhealthy", data["status"])
		
		checks := data["checks"].(map[string]interface{})
		assert.Equal(t, "unhealthy", checks["database"])
		
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	
	t.Run("unhealthy when database query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		
		// Expect ping to succeed
		mock.ExpectPing()
		
		// Expect SELECT 1 query to fail
		mock.ExpectQuery("SELECT 1").WillReturnError(sql.ErrConnDone)
		
		handler := NewHealthHandler(db, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		
		handler.HandleReadiness(w, req)
		
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		
		var response map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "unhealthy", data["status"])
		
		checks := data["checks"].(map[string]interface{})
		assert.Equal(t, "unhealthy", checks["database"])
		
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	
	t.Run("healthy when no database configured", func(t *testing.T) {
		handler := NewHealthHandler(nil, logger)
		
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		
		handler.HandleReadiness(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		
		data := response["data"].(map[string]interface{})
		assert.Equal(t, "healthy", data["status"])
		
		checks := data["checks"].(map[string]interface{})
		assert.Equal(t, "healthy", checks["database"])
	})
}

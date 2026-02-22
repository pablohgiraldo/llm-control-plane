package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/app"
	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/middleware"
	"github.com/upb/llm-control-plane/backend/routes"
	"go.uber.org/zap/zaptest"
)

// rejectAllValidator rejects all tokens for testing (unauthenticated requests get 401)
type rejectAllValidator struct{}

func (*rejectAllValidator) ValidateToken(context.Context, string) (*middleware.Claims, error) {
	return nil, assert.AnError
}

func TestMain(m *testing.M) {
	// Setup
	os.Setenv("ENVIRONMENT", "test")
	os.Setenv("LOG_LEVEL", "error")

	// Run tests
	code := m.Run()

	// Teardown
	os.Exit(code)
}

func TestInitLogger(t *testing.T) {
	t.Run("default json logger", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "info")
		os.Setenv("LOG_FORMAT", "json")

		logger, err := initLogger()
		require.NoError(t, err)
		require.NotNil(t, logger)
		defer logger.Sync()
	})

	t.Run("development console logger", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_FORMAT", "console")

		logger, err := initLogger()
		require.NoError(t, err)
		require.NotNil(t, logger)
		defer logger.Sync()
	})

	t.Run("invalid log level", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "invalid")
		os.Setenv("LOG_FORMAT", "json")

		logger, err := initLogger()
		assert.Error(t, err)
		assert.Nil(t, logger)
		assert.Contains(t, err.Error(), "invalid log level")
	})

	t.Run("defaults when not set", func(t *testing.T) {
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")

		logger, err := initLogger()
		require.NoError(t, err)
		require.NotNil(t, logger)
		defer logger.Sync()
	})
}

func TestApplicationStartup(t *testing.T) {
	t.Run("successful startup with mocked dependencies", func(t *testing.T) {
		// This tests the route setup with minimal dependencies
		cfg := testConfig(t)
		logger := zaptest.NewLogger(t)

		// Create mock dependencies (skip actual DB/Redis connection)
		deps := &app.Dependencies{
			Config: cfg,
			Logger: logger,
			ProviderRegistry: app.NewProviderRegistry(logger),
		}

		// Setup routes
		handler := routes.SetupRoutes(deps)
		require.NotNil(t, handler)

		// Create test server
		ts := httptest.NewServer(handler)
		defer ts.Close()

		// Test health check endpoint
		resp, err := http.Get(ts.URL + "/healthz")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "ok", body["status"])
	})
}

func TestHealthEndpoints(t *testing.T) {
	cfg := testConfig(t)
	logger := zaptest.NewLogger(t)

	// Create minimal dependencies for health check
	deps := &app.Dependencies{
		Config: cfg,
		Logger: logger,
		ProviderRegistry: app.NewProviderRegistry(logger),
	}

	handler := routes.SetupRoutes(deps)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("health check returns ok", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/healthz")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "ok", body["status"])
	})

	t.Run("status endpoint returns version info", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Contains(t, body, "version")
		assert.Contains(t, body, "environment")
		assert.Contains(t, body, "providers")
	})
}

func TestReadinessCheck(t *testing.T) {
	t.Run("not ready without infrastructure", func(t *testing.T) {
		cfg := testConfig(t)
		logger := zaptest.NewLogger(t)

		deps := &app.Dependencies{
			Config: cfg,
			Logger: logger,
			ProviderRegistry: app.NewProviderRegistry(logger),
		}

		handler := routes.SetupRoutes(deps)
		ts := httptest.NewServer(handler)
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/readyz")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 503 if DB/Redis not available
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "not_ready", body["status"])
	})
}

func TestAPIEndpoints(t *testing.T) {
	cfg := testConfig(t)
	logger := zaptest.NewLogger(t)

	deps := &app.Dependencies{
		Config:           cfg,
		Logger:           logger,
		ProviderRegistry: app.NewProviderRegistry(logger),
		AuthMiddleware:   middleware.NewAuthMiddleware(&rejectAllValidator{}, logger),
	}

	handler := routes.SetupRoutes(deps)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"chat completion", "POST", "/api/v1/inference/chat", http.StatusUnauthorized},
		{"list inference requests", "GET", "/api/v1/inference/requests", http.StatusUnauthorized},
		{"list organizations", "GET", "/api/v1/organizations", http.StatusUnauthorized},
		{"list applications", "GET", "/api/v1/applications", http.StatusUnauthorized},
		{"list policies", "GET", "/api/v1/policies", http.StatusUnauthorized},
		{"list audit logs", "GET", "/api/v1/audit/logs", http.StatusUnauthorized},
		{"list users", "GET", "/api/v1/users", http.StatusUnauthorized},
		{"get current user unauthenticated", "GET", "/api/v1/users/me", http.StatusUnauthorized},
		{"get metrics", "GET", "/api/v1/metrics/inference", http.StatusUnauthorized},
		{"not found", "GET", "/api/v1/nonexistent", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode, "endpoint: %s %s", tc.method, tc.path)
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	cfg := testConfig(t)
	logger := zaptest.NewLogger(t)

	deps := &app.Dependencies{
		Config: cfg,
		Logger: logger,
		ProviderRegistry: app.NewProviderRegistry(logger),
	}

	handler := routes.SetupRoutes(deps)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("OPTIONS preflight request", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", ts.URL+"/api/v1/status", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	})
}

func TestRequestIDMiddleware(t *testing.T) {
	cfg := testConfig(t)
	logger := zaptest.NewLogger(t)

	deps := &app.Dependencies{
		Config: cfg,
		Logger: logger,
		ProviderRegistry: app.NewProviderRegistry(logger),
	}

	handler := routes.SetupRoutes(deps)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Request should succeed (RequestID middleware is present, 
	// even if not exposed in response headers by default)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestIntegrationWithRealDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	cfg := testConfig(t)
	logger := zaptest.NewLogger(t)

	// Try to initialize real dependencies
	deps, err := app.NewDependencies(ctx, cfg, logger)
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
		return
	}
	defer deps.Close(ctx)

	// Setup routes with real dependencies
	handler := routes.SetupRoutes(deps)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	t.Run("readiness check with real infrastructure", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/readyz")
		require.NoError(t, err)
		defer resp.Body.Close()

		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		t.Logf("readiness response: %+v", body)

		// Should be ready with working infrastructure
		assert.Equal(t, "ready", body["status"])
		checks := body["checks"].(map[string]interface{})
		assert.Equal(t, "healthy", checks["database"])
	})
}

// Test helpers

func testConfig(t *testing.T) *config.Config {
	return &config.Config{
		Environment: "test",
		Server: config.ServerConfig{
			Host:            "localhost",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Database: config.DatabaseConfig{
			Host:            getEnvOrDefault("DB_HOST", "localhost"),
			Port:            5432,
			User:            getEnvOrDefault("DB_USER", "llmcp"),
			Password:        getEnvOrDefault("DB_PASSWORD", "llmcp"),
			Database:        getEnvOrDefault("DB_NAME", "llmcp_test"),
			SSLMode:         "disable",
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Cognito: config.CognitoConfig{
			Region:     "us-east-1",
			UserPoolID: "test-pool",
			ClientID:   "test-client",
		},
		Providers: config.ProvidersConfig{
			OpenAI: config.OpenAIConfig{
				APIKey:     "",
				BaseURL:    "https://api.openai.com/v1",
				Timeout:    60 * time.Second,
				MaxRetries: 3,
			},
		},
		Observability: config.ObservabilityConfig{
			LogLevel:       "error",
			LogFormat:      "json",
			MetricsEnabled: false,
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

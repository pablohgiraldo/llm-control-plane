package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upb/llm-control-plane/backend/config"
	"github.com/upb/llm-control-plane/backend/repositories/postgres"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNewDependencies(t *testing.T) {
	t.Run("successful initialization with all components", func(t *testing.T) {
		ctx := context.Background()
		cfg := testConfig(t)
		logger := zaptest.NewLogger(t)

		// Skip if database not available
		if !isDatabaseAvailable(t, cfg) {
			t.Skip("database not available")
		}

		deps, err := NewDependencies(ctx, cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, deps)

		// Verify infrastructure
		assert.NotNil(t, deps.Config)
		assert.NotNil(t, deps.DB)
		assert.NotNil(t, deps.Logger)

		// Verify repositories
		assert.NotNil(t, deps.Organizations)
		assert.NotNil(t, deps.Applications)
		assert.NotNil(t, deps.Users)
		assert.NotNil(t, deps.Policies)
		assert.NotNil(t, deps.AuditLogs)
		assert.NotNil(t, deps.InferenceRequests)
		assert.NotNil(t, deps.TxManager)

		// Verify provider registry
		assert.NotNil(t, deps.ProviderRegistry)

		// Cleanup
		err = deps.Close(ctx)
		assert.NoError(t, err)
	})

	t.Run("database connection failure", func(t *testing.T) {
		ctx := context.Background()
		cfg := testConfig(t)
		cfg.Database.Host = "invalid-host-that-does-not-exist"
		logger := zaptest.NewLogger(t)

		deps, err := NewDependencies(ctx, cfg, logger)
		assert.Error(t, err)
		assert.Nil(t, deps)
		assert.Contains(t, err.Error(), "failed to initialize database")
	})

}

func TestDependenciesClose(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		ctx := context.Background()
		cfg := testConfig(t)
		logger := zaptest.NewLogger(t)

		// Skip if database not available
		if !isDatabaseAvailable(t, cfg) {
			t.Skip("database not available")
		}

		deps, err := NewDependencies(ctx, cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, deps)

		// Close should succeed
		err = deps.Close(ctx)
		assert.NoError(t, err)

		// Second close should not panic
		err = deps.Close(ctx)
		// May error or not, but shouldn't panic
	})
}

func TestProviderRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("register and get provider", func(t *testing.T) {
		registry := NewProviderRegistry(logger)
		assert.Equal(t, 0, registry.Count())

		// Create a mock provider
		cfg := config.OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
		}
		provider := NewOpenAIAdapter(cfg, logger)

		// Register
		registry.Register(provider)
		assert.Equal(t, 1, registry.Count())

		// Get
		retrieved, ok := registry.Get("openai")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "openai", retrieved.Name())
	})

	t.Run("get non-existent provider", func(t *testing.T) {
		registry := NewProviderRegistry(logger)

		provider, ok := registry.Get("non-existent")
		assert.False(t, ok)
		assert.Nil(t, provider)
	})

	t.Run("list providers", func(t *testing.T) {
		registry := NewProviderRegistry(logger)

		// Empty registry
		names := registry.List()
		assert.Empty(t, names)

		// Add provider
		cfg := config.OpenAIConfig{APIKey: "test"}
		provider := NewOpenAIAdapter(cfg, logger)
		registry.Register(provider)

		names = registry.List()
		assert.Len(t, names, 1)
		assert.Contains(t, names, "openai")
	})
}

func TestOpenAIAdapter(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("adapter name", func(t *testing.T) {
		cfg := config.OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
		}
		adapter := NewOpenAIAdapter(cfg, logger)

		assert.Equal(t, "openai", adapter.Name())
	})

	t.Run("is available with api key", func(t *testing.T) {
		cfg := config.OpenAIConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
		}
		adapter := NewOpenAIAdapter(cfg, logger)

		assert.True(t, adapter.IsAvailable(context.Background()))
	})

	t.Run("is not available without api key", func(t *testing.T) {
		cfg := config.OpenAIConfig{
			APIKey:  "",
			BaseURL: "https://api.openai.com/v1",
		}
		adapter := NewOpenAIAdapter(cfg, logger)

		assert.False(t, adapter.IsAvailable(context.Background()))
	})

	t.Run("chat completion not implemented", func(t *testing.T) {
		cfg := config.OpenAIConfig{APIKey: "test"}
		adapter := NewOpenAIAdapter(cfg, logger)

		resp, err := adapter.ChatCompletion(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "not implemented")
	})

	t.Run("calculate cost not implemented", func(t *testing.T) {
		cfg := config.OpenAIConfig{APIKey: "test"}
		adapter := NewOpenAIAdapter(cfg, logger)

		cost, err := adapter.CalculateCost(nil, nil)
		assert.Error(t, err)
		assert.Equal(t, 0.0, cost)
		assert.Contains(t, err.Error(), "not implemented")
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
			ShutdownTimeout: 10 * time.Second,
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
				APIKey:     getEnvOrDefault("OPENAI_API_KEY", ""),
				BaseURL:    "https://api.openai.com/v1",
				Timeout:    60 * time.Second,
				MaxRetries: 3,
			},
		},
		Observability: config.ObservabilityConfig{
			LogLevel:       "debug",
			LogFormat:      "json",
			MetricsEnabled: false,
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	// In tests, just return default
	return defaultValue
}

func isDatabaseAvailable(t *testing.T, cfg *config.Config) bool {
	logger := zap.NewNop()
	factory, err := postgres.NewRepositoryFactory(cfg, logger)
	if err != nil {
		return false
	}
	defer factory.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return factory.GetDB().PingContext(ctx) == nil
}

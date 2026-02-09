package runtimeconfig

import "context"

// Manager provides access to runtime configuration.
type Manager interface {
	Get(ctx context.Context, key string) (string, error)
	GetWithDefault(ctx context.Context, key, defaultValue string) string
	Reload(ctx context.Context) error
}

// Config represents the application configuration.
type Config struct {
	Environment string
	HTTPAddr    string
	Database    DatabaseConfig
	Redis       RedisConfig
	Providers   ProvidersConfig
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL             string
	MaxConnections  int
	ConnMaxLifetime int
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

// ProvidersConfig holds LLM provider credentials.
type ProvidersConfig struct {
	OpenAI    ProviderCredentials
	Anthropic ProviderCredentials
	Azure     AzureCredentials
}

// ProviderCredentials holds API credentials for a provider.
type ProviderCredentials struct {
	APIKey string
}

// AzureCredentials holds Azure-specific credentials.
type AzureCredentials struct {
	APIKey   string
	Endpoint string
}

// TODO: Implement configuration loader with hot-reload support

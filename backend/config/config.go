package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config represents the complete application configuration
type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	AuditDatabase  *DatabaseConfig // Optional: separate DB for audit logs. When nil, audit uses main DB.
	Cognito        CognitoConfig
	Providers      ProvidersConfig
	Observability  ObservabilityConfig
	Environment    string
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	TLS             struct {
		Enabled  bool
		CertFile string
		KeyFile  string
	}
}

// DatabaseConfig holds PostgreSQL database configuration.
// When ConnectionString (from DATABASE_URL) is set, it takes precedence over individual fields.
type DatabaseConfig struct {
	ConnectionString string // From DATABASE_URL when set
	Host             string
	Port             int
	User             string
	Password         string
	Database         string
	SSLMode          string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
}

// CognitoConfig holds AWS Cognito authentication configuration
type CognitoConfig struct {
	Region       string
	UserPoolID   string
	ClientID     string
	ClientSecret string
	Domain       string // Cognito domain (e.g., https://my-app.auth.us-east-1.amazoncognito.com)
	RedirectURI  string // OAuth2 callback URL
	FrontEndURL  string // Post-login redirect target (loaded from FRONT_END_URL)
}

// ProvidersConfig holds LLM provider configurations
type ProvidersConfig struct {
	OpenAI    OpenAIConfig
	Anthropic AnthropicConfig
	Bedrock   BedrockConfig
}

// OpenAIConfig holds OpenAI provider configuration
type OpenAIConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// AnthropicConfig holds Anthropic provider configuration
type AnthropicConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
}

// BedrockConfig holds AWS Bedrock provider configuration
type BedrockConfig struct {
	Region     string
	AccessKey  string
	SecretKey  string
	Timeout    time.Duration
	MaxRetries int
}

// ObservabilityConfig holds monitoring and logging configuration
type ObservabilityConfig struct {
	LogLevel          string
	LogFormat         string // json or text
	MetricsEnabled    bool
	MetricsPort       int
	TracingEnabled    bool
	TracingEndpoint   string
	TracingSampleRate float64
}

// New creates a new Config instance by loading environment variables
func New(ctx context.Context) (*Config, error) {
	// Load .env file if it exists (backend/.env when run from project root, .env when run from backend/)
	_ = godotenv.Load("backend/.env")
	_ = godotenv.Load(".env")

	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getPort(),
			ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
			TLS: struct {
				Enabled  bool
				CertFile string
				KeyFile  string
			}{
				Enabled:  getEnvAsBool("TLS_ENABLED", true),
				CertFile: getEnv("TLS_CERT_FILE", "certs/cert.pem"),
				KeyFile:  getEnv("TLS_KEY_FILE", "certs/key.pem"),
			},
		},
		Database:      loadDatabaseConfig(),
		AuditDatabase: loadAuditDatabaseConfig(),
		Cognito: CognitoConfig{
			Region:       getEnv("COGNITO_REGION", "us-east-1"),
			UserPoolID:   getEnv("COGNITO_USER_POOL_ID", ""),
			ClientID:     getEnv("COGNITO_CLIENT_ID", ""),
			ClientSecret: getEnv("COGNITO_CLIENT_SECRET", ""),
			Domain:       getEnv("COGNITO_DOMAIN", ""),
			RedirectURI:  getEnv("COGNITO_REDIRECT_URI", "https://localhost:8443/oauth2/idpresponse"),
			FrontEndURL:  getEnv("FRONT_END_URL", "http://localhost:5173"),
		},
		Providers: ProvidersConfig{
			OpenAI: OpenAIConfig{
				APIKey:     getEnv("OPENAI_API_KEY", ""),
				BaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
				Timeout:    getEnvAsDuration("OPENAI_TIMEOUT", 60*time.Second),
				MaxRetries: getEnvAsInt("OPENAI_MAX_RETRIES", 3),
			},
			Anthropic: AnthropicConfig{
				APIKey:     getEnv("ANTHROPIC_API_KEY", ""),
				BaseURL:    getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
				Timeout:    getEnvAsDuration("ANTHROPIC_TIMEOUT", 60*time.Second),
				MaxRetries: getEnvAsInt("ANTHROPIC_MAX_RETRIES", 3),
			},
			Bedrock: BedrockConfig{
				Region:     getEnv("BEDROCK_REGION", "us-east-1"),
				AccessKey:  getEnv("BEDROCK_ACCESS_KEY", ""),
				SecretKey:  getEnv("BEDROCK_SECRET_KEY", ""),
				Timeout:    getEnvAsDuration("BEDROCK_TIMEOUT", 60*time.Second),
				MaxRetries: getEnvAsInt("BEDROCK_MAX_RETRIES", 3),
			},
		},
		Observability: ObservabilityConfig{
			LogLevel:          getEnv("LOG_LEVEL", "info"),
			LogFormat:         getEnv("LOG_FORMAT", "json"),
			MetricsEnabled:    getEnvAsBool("METRICS_ENABLED", true),
			MetricsPort:       getEnvAsInt("METRICS_PORT", 9090),
			TracingEnabled:    getEnvAsBool("TRACING_ENABLED", false),
			TracingEndpoint:   getEnv("TRACING_ENDPOINT", ""),
			TracingSampleRate: getEnvAsFloat("TRACING_SAMPLE_RATE", 0.1),
		},
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks if all required configuration fields are set
func (c *Config) Validate() error {
	// Database validation (DATABASE_URL or DB_* vars)
	if c.Database.ConnectionString == "" && c.Database.Host == "" {
		return fmt.Errorf("database configuration required: set DATABASE_URL or DB_HOST")
	}
	if c.Database.ConnectionString == "" {
		if c.Database.User == "" {
			return fmt.Errorf("database user is required")
		}
		if c.Database.Database == "" {
			return fmt.Errorf("database name is required")
		}
	}

	// Cognito validation (required in production)
	if c.IsProduction() {
		if c.Cognito.UserPoolID == "" {
			return fmt.Errorf("cognito user pool ID is required in production")
		}
		if c.Cognito.ClientID == "" {
			return fmt.Errorf("cognito client ID is required in production")
		}
	}

	// Provider validation (at least one provider API key required in production)
	if c.IsProduction() {
		if c.Providers.OpenAI.APIKey == "" &&
			c.Providers.Anthropic.APIKey == "" &&
			c.Providers.Bedrock.AccessKey == "" {
			return fmt.Errorf("at least one LLM provider must be configured in production")
		}
	}

	// Observability validation
	if c.Observability.LogLevel == "" {
		return fmt.Errorf("log level is required")
	}

	return nil
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

// DSN returns the PostgreSQL connection string.
// Uses ConnectionString (from DATABASE_URL) when set; otherwise builds from individual fields.
func (c *DatabaseConfig) DSN() string {
	if c.ConnectionString != "" {
		return c.ConnectionString
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// LogString returns a safe string for logging (no password). Parses ConnectionString when set.
func (c *DatabaseConfig) LogString() string {
	if c.ConnectionString != "" {
		u, err := url.Parse(c.ConnectionString)
		if err == nil {
			host := u.Hostname()
			port := u.Port()
			if port == "" {
				port = "5432"
			}
			db := strings.TrimPrefix(u.Path, "/")
			return fmt.Sprintf("host=%s port=%s database=%s", host, port, db)
		}
		return "host=<from DATABASE_URL>"
	}
	return fmt.Sprintf("host=%s port=%d database=%s", c.Host, c.Port, c.Database)
}

// loadDatabaseConfig loads database config from DATABASE_URL or DB_* env vars (tsum-app pattern)
func loadDatabaseConfig() DatabaseConfig {
	dbURL := getEnv("DATABASE_URL", "")
	if dbURL != "" {
		return DatabaseConfig{
			ConnectionString: dbURL,
			MaxOpenConns:     getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:     getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime:  getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		}
	}
	return DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvAsInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "dev"),
		Password:        getEnv("DB_PASSWORD", "audit_password"),
		Database:        getEnv("DB_NAME", "audit"),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
	}
}

// loadAuditDatabaseConfig loads audit DB config from DATABASE_URL_AUDIT.
// Returns nil when not set (audit uses main DB).
func loadAuditDatabaseConfig() *DatabaseConfig {
	dbURL := getEnv("DATABASE_URL_AUDIT", "")
	if dbURL == "" {
		return nil
	}
	return &DatabaseConfig{
		ConnectionString: dbURL,
		MaxOpenConns:     getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:     getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime:  getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
	}
}

// Address returns the HTTP server address
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Helper functions

// getPort returns the server port from PORT or SERVER_PORT env vars (default: 8443)
func getPort() int {
	if value := os.Getenv("PORT"); value != "" {
		if p, err := strconv.Atoi(value); err == nil {
			return p
		}
	}
	if value := os.Getenv("SERVER_PORT"); value != "" {
		if p, err := strconv.Atoi(value); err == nil {
			return p
		}
	}
	return 8443
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

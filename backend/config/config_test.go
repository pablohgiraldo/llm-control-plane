package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "default configuration",
			envVars: map[string]string{
				"ENVIRONMENT": "development",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "development", cfg.Environment)
				assert.Equal(t, "0.0.0.0", cfg.Server.Host)
				assert.Equal(t, 8443, cfg.Server.Port)
				assert.True(t, cfg.Server.TLS.Enabled)
				assert.Equal(t, "certs/cert.pem", cfg.Server.TLS.CertFile)
				assert.Equal(t, "certs/key.pem", cfg.Server.TLS.KeyFile)
				assert.Equal(t, "localhost", cfg.Database.Host)
				assert.Equal(t, 5432, cfg.Database.Port)
				assert.Equal(t, "dev", cfg.Database.User)
			},
		},
		{
			name: "production configuration with all providers",
			envVars: map[string]string{
				"ENVIRONMENT":         "production",
				"SERVER_PORT":         "9000",
				"DB_HOST":             "prod-db.example.com",
				"DB_PORT":             "5433",
				"COGNITO_USER_POOL_ID": "us-east-1_xxxxx",
				"COGNITO_CLIENT_ID":    "client123",
				"OPENAI_API_KEY":       "sk-xxxxx",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.IsProduction())
				assert.False(t, cfg.IsDevelopment())
				assert.Equal(t, 9000, cfg.Server.Port)
				assert.Equal(t, "prod-db.example.com", cfg.Database.Host)
				assert.Equal(t, 5433, cfg.Database.Port)
				assert.NotEmpty(t, cfg.Cognito.UserPoolID)
				assert.NotEmpty(t, cfg.Providers.OpenAI.APIKey)
			},
		},
		{
			name: "custom timeouts and pool settings",
			envVars: map[string]string{
				"SERVER_READ_TIMEOUT":  "60s",
				"SERVER_WRITE_TIMEOUT": "90s",
				"DB_MAX_OPEN_CONNS":    "50",
				"DB_MAX_IDLE_CONNS":   "10",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
				assert.Equal(t, 90*time.Second, cfg.Server.WriteTimeout)
				assert.Equal(t, 50, cfg.Database.MaxOpenConns)
				assert.Equal(t, 10, cfg.Database.MaxIdleConns)
			},
		},
		{
			name: "observability configuration",
			envVars: map[string]string{
				"LOG_LEVEL":            "debug",
				"LOG_FORMAT":           "text",
				"METRICS_ENABLED":      "true",
				"METRICS_PORT":         "9091",
				"TRACING_ENABLED":      "true",
				"TRACING_ENDPOINT":     "http://jaeger:14268",
				"TRACING_SAMPLE_RATE":  "0.5",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "debug", cfg.Observability.LogLevel)
				assert.Equal(t, "text", cfg.Observability.LogFormat)
				assert.True(t, cfg.Observability.MetricsEnabled)
				assert.Equal(t, 9091, cfg.Observability.MetricsPort)
				assert.True(t, cfg.Observability.TracingEnabled)
				assert.Equal(t, "http://jaeger:14268", cfg.Observability.TracingEndpoint)
				assert.Equal(t, 0.5, cfg.Observability.TracingSampleRate)
			},
		},
		{
			name: "TLS configuration defaults",
			envVars: map[string]string{
				"ENVIRONMENT": "development",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.True(t, cfg.Server.TLS.Enabled)
				assert.Equal(t, "certs/cert.pem", cfg.Server.TLS.CertFile)
				assert.Equal(t, "certs/key.pem", cfg.Server.TLS.KeyFile)
			},
		},
		{
			name: "TLS configuration overrides",
			envVars: map[string]string{
				"ENVIRONMENT":   "development",
				"TLS_ENABLED":  "false",
				"TLS_CERT_FILE": "/etc/ssl/certs/server.crt",
				"TLS_KEY_FILE":  "/etc/ssl/private/server.key",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.False(t, cfg.Server.TLS.Enabled)
				assert.Equal(t, "/etc/ssl/certs/server.crt", cfg.Server.TLS.CertFile)
				assert.Equal(t, "/etc/ssl/private/server.key", cfg.Server.TLS.KeyFile)
			},
		},
		{
			name: "PORT env var takes precedence over SERVER_PORT default",
			envVars: map[string]string{
				"ENVIRONMENT": "development",
				"PORT":        "9443",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 9443, cfg.Server.Port)
			},
		},
		{
			name: "SERVER_PORT env var when PORT not set",
			envVars: map[string]string{
				"ENVIRONMENT":  "development",
				"SERVER_PORT": "9000",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 9000, cfg.Server.Port)
			},
		},
		{
			name: "Cognito FrontEndURL from FRONT_END_URL env",
			envVars: map[string]string{
				"ENVIRONMENT":   "development",
				"FRONT_END_URL": "http://localhost:5173",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "http://localhost:5173", cfg.Cognito.FrontEndURL)
			},
		},
		{
			name: "Cognito RedirectURI default",
			envVars: map[string]string{
				"ENVIRONMENT": "development",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "https://localhost:8443/oauth2/idpresponse", cfg.Cognito.RedirectURI)
			},
		},
		{
			name: "production without cognito config",
			envVars: map[string]string{
				"ENVIRONMENT": "production",
			},
			wantErr: true,
		},
		{
			name: "production without any provider",
			envVars: map[string]string{
				"ENVIRONMENT":          "production",
				"COGNITO_USER_POOL_ID": "pool-id",
				"COGNITO_CLIENT_ID":    "client-id",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Create config
			cfg, err := New(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid development config",
			config: &Config{
				Environment: "development",
				Database: DatabaseConfig{
					Host:     "localhost",
					User:     "user",
					Database: "db",
				},
				Observability: ObservabilityConfig{
					LogLevel: "info",
				},
			},
			wantErr: false,
		},
		{
			name: "missing database host",
			config: &Config{
				Environment: "development",
				Database: DatabaseConfig{
					Host:     "",
					User:     "user",
					Database: "db",
				},
				Observability: ObservabilityConfig{
					LogLevel: "info",
				},
			},
			wantErr: true,
			errMsg:  "database configuration required",
		},
		{
			name: "missing database user",
			config: &Config{
				Environment: "development",
				Database: DatabaseConfig{
					Host:     "localhost",
					User:     "",
					Database: "db",
				},
				Observability: ObservabilityConfig{
					LogLevel: "info",
				},
			},
			wantErr: true,
			errMsg:  "database user is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		want        bool
	}{
		{"production", "production", true},
		{"prod", "prod", true},
		{"development", "development", false},
		{"dev", "dev", false},
		{"staging", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.environment}
			assert.Equal(t, tt.want, cfg.IsProduction())
		})
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		want        bool
	}{
		{"development", "development", true},
		{"dev", "dev", true},
		{"production", "production", false},
		{"staging", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.environment}
			assert.Equal(t, tt.want, cfg.IsDevelopment())
		})
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	assert.Equal(t, expected, cfg.DSN())
}

func TestServerConfig_Address(t *testing.T) {
	cfg := ServerConfig{
		Host: "0.0.0.0",
		Port: 8443,
	}

	assert.Equal(t, "0.0.0.0:8443", cfg.Address())
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue int
		want         int
	}{
		{"valid int", "TEST_INT", "42", 10, 42},
		{"empty value", "TEST_INT", "", 10, 10},
		{"invalid int", "TEST_INT", "not-a-number", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			}
			got := getEnvAsInt(tt.key, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue bool
		want         bool
	}{
		{"true", "TEST_BOOL", "true", false, true},
		{"false", "TEST_BOOL", "false", true, false},
		{"empty value", "TEST_BOOL", "", true, true},
		{"invalid bool", "TEST_BOOL", "not-a-bool", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			}
			got := getEnvAsBool(tt.key, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvAsFloat(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue float64
		want         float64
	}{
		{"valid float", "TEST_FLOAT", "3.14", 1.0, 3.14},
		{"empty value", "TEST_FLOAT", "", 1.0, 1.0},
		{"invalid float", "TEST_FLOAT", "not-a-number", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			}
			got := getEnvAsFloat(tt.key, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue time.Duration
		want         time.Duration
	}{
		{"valid duration", "TEST_DURATION", "30s", 10 * time.Second, 30 * time.Second},
		{"empty value", "TEST_DURATION", "", 10 * time.Second, 10 * time.Second},
		{"invalid duration", "TEST_DURATION", "not-a-duration", 10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
			}
			got := getEnvAsDuration(tt.key, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

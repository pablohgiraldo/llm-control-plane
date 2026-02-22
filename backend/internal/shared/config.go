package shared

import "os"

// Config holds environment-driven settings.
// TODO: extend with structured sub-configs and validation.
type Config struct {
	HTTPAddr    string
	PostgresDSN string
	Environment string
}

func LoadConfig() Config {
	return Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://llmcp:llmcp@localhost:5432/llmcp?sslmode=disable"),
		Environment: getEnv("ENV", "dev"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}


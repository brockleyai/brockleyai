// Package config provides application configuration loaded from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	// Database and cache
	DatabaseURL string
	RedisURL    string

	// Server
	ServerHost string
	ServerPort string
	Env        string // "development" or "production"

	// Logging
	LogLevel  string
	LogFormat string

	// Security
	APIKeys       []string
	CORSOrigins   string
	EncryptionKey string

	// Observability
	MetricsEnabled bool

	// Trace export configuration (OpenInference-compatible)
	TraceEnabled  bool
	TraceEndpoint string
	TraceAPIKey   string
	TraceProject  string
	TraceProtocol string
	TraceInsecure bool

	// Multi-tenancy
	Namespace string
}

// Load reads configuration from environment variables, applying defaults.
func Load() *Config {
	c := &Config{
		DatabaseURL:   getEnv("BROCKLEY_DATABASE_URL", ""),
		RedisURL:      getEnv("BROCKLEY_REDIS_URL", ""),
		ServerHost:    getEnv("BROCKLEY_HOST", "0.0.0.0"),
		ServerPort:    getEnv("BROCKLEY_PORT", "8000"),
		Env:           getEnv("BROCKLEY_ENV", "production"),
		LogLevel:      getEnv("BROCKLEY_LOG_LEVEL", "info"),
		LogFormat:     getEnv("BROCKLEY_LOG_FORMAT", "json"),
		CORSOrigins:   getEnv("BROCKLEY_CORS_ORIGINS", ""),
		EncryptionKey: getEnv("BROCKLEY_ENCRYPTION_KEY", ""),

		MetricsEnabled: getEnv("BROCKLEY_METRICS_ENABLED", "") == "true",

		TraceEnabled:  getEnv("BROCKLEY_TRACE_ENABLED", "") == "true",
		TraceEndpoint: getEnv("BROCKLEY_TRACE_ENDPOINT", ""),
		TraceAPIKey:   getEnv("BROCKLEY_TRACE_API_KEY", ""),
		TraceProject:  getEnv("BROCKLEY_TRACE_PROJECT", ""),
		TraceProtocol: getEnv("BROCKLEY_TRACE_PROTOCOL", ""),
		TraceInsecure: getEnv("BROCKLEY_TRACE_INSECURE", "") == "true",

		Namespace: getEnv("BROCKLEY_NAMESPACE", "default"),
	}

	if keys := getEnv("BROCKLEY_API_KEYS", ""); keys != "" {
		c.APIKeys = strings.Split(keys, ",")
		for i := range c.APIKeys {
			c.APIKeys[i] = strings.TrimSpace(c.APIKeys[i])
		}
	}

	return c
}

// IsDevelopment returns true when the environment is set to development.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

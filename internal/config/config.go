// Package config loads application configuration from environment variables.
// It provides a single Load() function that returns a Config struct with
// sensible defaults for local development.
package config

import "os"

// Config holds the application-level configuration values sourced from
// environment variables.
type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	EnableIntrospection bool
}

// Load reads environment variables and returns a populated Config.
// Missing optional variables fall back to sensible defaults.
func Load() Config {
	return Config{
		Port:                getEnv("PORT", "4000"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		EnableIntrospection: os.Getenv("ENABLE_INTROSPECTION") == "true",
	}
}

// getEnv returns the value of the environment variable named by key, or
// fallback if the variable is not set.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

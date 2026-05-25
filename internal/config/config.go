package config

import "os"

type Config struct {
	Port                string
	DatabaseURL         string
	JWTSecret           string
	EnableIntrospection bool
}

func Load() Config {
	return Config{
		Port:                getEnv("PORT", "4000"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		EnableIntrospection: os.Getenv("ENABLE_INTROSPECTION") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

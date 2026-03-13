package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application settings loaded from environment variables.
type Config struct {
	DatabaseURL       string
	SessionTimeout    time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
}

// Load reads settings from environment variables and falls back to safe defaults.
func Load() *Config {
	return &Config{
		DatabaseURL:       getEnv("DB_URL", "postgres://user:pass@localhost:5432/loginapp?sslmode=disable"),
		SessionTimeout:    getDuration("SESSION_TIMEOUT", 30*time.Minute),
		MaxFailedAttempts: getInt("MAX_FAILED_ATTEMPTS", 5),
		LockoutDuration:   getDuration("LOCKOUT_DURATION", 15*time.Minute),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

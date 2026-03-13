package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Use t.Setenv — it automatically restores the original value after the test
	t.Setenv("DB_URL", "")
	t.Setenv("SESSION_TIMEOUT", "")
	t.Setenv("MAX_FAILED_ATTEMPTS", "")
	t.Setenv("LOCKOUT_DURATION", "")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/loginapp?sslmode=disable" {
		t.Errorf("unexpected default DB_URL: %s", cfg.DatabaseURL)
	}
	if cfg.SessionTimeout != 30*time.Minute {
		t.Errorf("unexpected default SESSION_TIMEOUT: %v", cfg.SessionTimeout)
	}
	if cfg.MaxFailedAttempts != 5 {
		t.Errorf("unexpected default MAX_FAILED_ATTEMPTS: %d", cfg.MaxFailedAttempts)
	}
	if cfg.LockoutDuration != 15*time.Minute {
		t.Errorf("unexpected default LOCKOUT_DURATION: %v", cfg.LockoutDuration)
	}
}

func TestLoad_FromEnvVars(t *testing.T) {
	t.Setenv("DB_URL", "postgres://testuser:testpass@db:5432/testdb?sslmode=disable")
	t.Setenv("SESSION_TIMEOUT", "1h")
	t.Setenv("MAX_FAILED_ATTEMPTS", "3")
	t.Setenv("LOCKOUT_DURATION", "5m")

	cfg := Load()

	if cfg.DatabaseURL != "postgres://testuser:testpass@db:5432/testdb?sslmode=disable" {
		t.Errorf("unexpected DB_URL: %s", cfg.DatabaseURL)
	}
	if cfg.SessionTimeout != time.Hour {
		t.Errorf("unexpected SESSION_TIMEOUT: %v", cfg.SessionTimeout)
	}
	if cfg.MaxFailedAttempts != 3 {
		t.Errorf("unexpected MAX_FAILED_ATTEMPTS: %d", cfg.MaxFailedAttempts)
	}
	if cfg.LockoutDuration != 5*time.Minute {
		t.Errorf("unexpected LOCKOUT_DURATION: %v", cfg.LockoutDuration)
	}
}

func TestLoad_InvalidDurationFallsBackToDefault(t *testing.T) {
	t.Setenv("SESSION_TIMEOUT", "not-a-duration")

	cfg := Load()

	if cfg.SessionTimeout != 30*time.Minute {
		t.Errorf("expected fallback to default on invalid duration, got: %v", cfg.SessionTimeout)
	}
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("MAX_FAILED_ATTEMPTS", "abc")

	cfg := Load()

	if cfg.MaxFailedAttempts != 5 {
		t.Errorf("expected fallback to default on invalid int, got: %d", cfg.MaxFailedAttempts)
	}
}

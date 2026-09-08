package config

import (
	"testing"
	"time"
)

func TestGetRequiredEnv(t *testing.T) {
	t.Setenv("TEST_KEY", "test-value")

	value, err := getRequiredEnv("TEST_KEY")
	if err != nil {
		t.Fatalf("failed to get required env: %v", err)
	}

	if value != "test-value" {
		t.Fatalf("expected value to be 'test-value', got '%s'", value)
	}
}

func TestGetRequiredEnv_Missing(t *testing.T) {
	t.Setenv("TEST_MISSING", "")

	value, err := getRequiredEnv("TEST_MISSING")

	if err == nil {
		t.Fatalf("expected error to be non-nil, got nil")
	}

	if value != "" {
		t.Fatalf("expected value to be empty, got '%s'", value)
	}
}

func TestNewConfig(t *testing.T) {
	t.Setenv("HH_TOKEN", "test-token")
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("SEARCH_TEXT", "golang")
	t.Setenv("APPLY_INTERVAL", "30s")

	cfg, err := NewConfig()

	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	if cfg.HHToken != "test-token" {
		t.Fatalf("expected HHToken to be 'test-token', got '%v'", cfg.HHToken)
	}

	if cfg.AppName != "test-app" {
		t.Fatalf("expected AppName to be 'test-app', got '%v'", cfg.AppName)
	}

	if cfg.SearchText != "golang" {
		t.Fatalf("expected SearchText to be 'golang', got '%v'", cfg.SearchText)
	}

	if cfg.ApplyInterval != 30*time.Second {
		t.Fatalf("expected ApplyInterval to be 30 seconds, got '%v'", cfg.ApplyInterval)
	}
}

func TestNewConfig_MissingToken(t *testing.T) {
	t.Setenv("HH_TOKEN", "")
	t.Setenv("SEARCH_TEXT", "golang")
	t.Setenv("APPLY_INTERVAL", "30s")

	_, err := NewConfig()

	if err == nil {
		t.Fatal("expected error for missing HH_TOKEN, got nil")
	}
}

func TestNewConfig_InvalidInterval(t *testing.T) {
	t.Setenv("HH_TOKEN", "test-token")
	t.Setenv("SEARCH_TEXT", "golang")
	t.Setenv("APPLY_INTERVAL", "abc")

	_, err := NewConfig()

	if err == nil {
		t.Fatal("expected error for invalid APPLY_INTERVAL, got nil")
	}
}

func TestNewConfig_MissingAppName(t *testing.T) {
	t.Setenv("HH_TOKEN", "test-token")
	t.Setenv("SEARCH_TEXT", "golang")
	t.Setenv("APPLY_INTERVAL", "30s")

	cfg, err := NewConfig()

	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	if cfg.AppName != "HH Auto Apply" {
		t.Fatalf("expected AppName to be 'HH Auto Apply', got '%v'", cfg.AppName)
	}
}
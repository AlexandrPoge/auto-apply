package config

import (
	"testing"
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

	value ,err := getRequiredEnv("TEST_MISSING")

	if err == nil {
		t.Fatalf("expected error to be non-nil, got nil")
	}

	if value != "" {
		t.Fatalf("expected value to be empty, got '%s'", value)
	}
}

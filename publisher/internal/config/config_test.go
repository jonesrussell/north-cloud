package config

import (
	"os"
	"testing"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"true uppercase", "TRUE", true},
		{"true mixed case", "True", true},
		{"one", "1", true},
		{"yes lowercase", "yes", true},
		{"yes uppercase", "YES", true},
		{"yes mixed case", "Yes", true},
		{"false lowercase", "false", false},
		{"false uppercase", "FALSE", false},
		{"zero", "0", false},
		{"no lowercase", "no", false},
		{"no uppercase", "NO", false},
		{"empty string", "", false},
		{"whitespace true", "  true  ", true},
		{"whitespace false", "  false  ", false},
		{"invalid value", "maybe", false},
		{"invalid value 2", "2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigDebugFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true from env", "true", true},
		{"1 from env", "1", true},
		{"yes from env", "yes", true},
		{"false from env", "false", false},
		{"0 from env", "0", false},
		{"no env var", "", false}, // Should default to false
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testConfigDebugWithEnv(t, tt.envValue, tt.expected)
		})
	}
}

// testConfigDebugWithEnv tests config debug loading with a specific env value
func testConfigDebugWithEnv(t *testing.T, envValue string, expected bool) {
	t.Helper()

	// Save original value and restore after test
	//nolint:forbidigo // Test requires saving/restoring env var for proper cleanup
	originalValue := os.Getenv("APP_DEBUG")
	defer func() {
		if originalValue != "" {
			t.Setenv("APP_DEBUG", originalValue)
		} else {
			os.Unsetenv("APP_DEBUG")
		}
	}()

	if envValue != "" {
		t.Setenv("APP_DEBUG", envValue)
	} else {
		// Unset the environment variable for this test
		os.Unsetenv("APP_DEBUG")
	}

	// Create a minimal config and use infrastructure/config to load env vars
	tempFile, err := os.CreateTemp(t.TempDir(), "config_test_*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write minimal config
	_, err = tempFile.WriteString("debug: false\nserver:\n  address: \":8070\"\nservice:\n  check_interval: \"5m\"\n")
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tempFile.Close()

	// Load config using infrastructure/config which will apply env overrides
	cfg, err := infraconfig.LoadWithDefaults(tempFile.Name(), setTestDefaults)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Debug != expected {
		t.Errorf("Config.Debug = %v, want %v (APP_DEBUG=%q)", cfg.Debug, expected, envValue)
	}
}

// setTestDefaults sets default values for test config
func setTestDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8070"
	}
	if cfg.Service.CheckInterval == 0 {
		cfg.Service.CheckInterval = 5 * time.Minute
	}
}

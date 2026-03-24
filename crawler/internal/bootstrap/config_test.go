package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestNormalizeLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty returns info", "", "info"},
		{"lowercase unchanged", "debug", "debug"},
		{"uppercase lowered", "DEBUG", "debug"},
		{"mixed case lowered", "Warning", "warning"},
		{"info unchanged", "info", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := bootstrap.NormalizeLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeLogLevel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCommandDeps_Validate_NilLogger(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: nil,
		Config: nil,
	}

	err := deps.Validate()
	if err == nil {
		t.Fatal("expected error for nil logger, got nil")
	}

	const expectedMsg = "logger is required"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestCommandDeps_Validate_NilConfig(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: nil,
	}

	err := deps.Validate()
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}

	const expectedMsg = "config is required"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestCommandDeps_Validate_Success(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	err := deps.Validate()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

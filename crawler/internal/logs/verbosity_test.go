package logs_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestVerbosityParse(t *testing.T) {
	t.Helper()

	tests := []struct {
		input    string
		expected logs.Verbosity
		valid    bool
	}{
		{"quiet", logs.VerbosityQuiet, true},
		{"normal", logs.VerbosityNormal, true},
		{"debug", logs.VerbosityDebug, true},
		{"trace", logs.VerbosityTrace, true},
		{"NORMAL", logs.VerbosityNormal, true},
		{"invalid", "", false},
		{"", logs.VerbosityNormal, true}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := logs.ParseVerbosity(tt.input)
			if tt.valid {
				if err != nil {
					t.Errorf("ParseVerbosity(%q) unexpected error: %v", tt.input, err)
				}
				if v != tt.expected {
					t.Errorf("ParseVerbosity(%q) = %q, want %q", tt.input, v, tt.expected)
				}
			} else if err == nil {
				t.Errorf("ParseVerbosity(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestVerbosityAllowsLevel(t *testing.T) {
	t.Helper()

	tests := []struct {
		verbosity logs.Verbosity
		level     string
		allowed   bool
	}{
		{logs.VerbosityQuiet, "info", true},
		{logs.VerbosityQuiet, "debug", false},
		{logs.VerbosityNormal, "info", true},
		{logs.VerbosityNormal, "debug", false},
		{logs.VerbosityDebug, "info", true},
		{logs.VerbosityDebug, "debug", true},
		{logs.VerbosityTrace, "debug", true},
	}

	for _, tt := range tests {
		name := string(tt.verbosity) + "_" + tt.level
		t.Run(name, func(t *testing.T) {
			if got := tt.verbosity.AllowsLevel(tt.level); got != tt.allowed {
				t.Errorf("%s.AllowsLevel(%q) = %v, want %v", tt.verbosity, tt.level, got, tt.allowed)
			}
		})
	}
}

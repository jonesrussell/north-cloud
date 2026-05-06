package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// configYMLPath is the path to the shipped config.yml relative to the repo root.
// Tests in this file are run from the alert-crawler module root.
const configYMLPath = "../../config.yml"

// setDefaultsOwnedPaths lists the YAML key paths that SetDefaults exclusively
// manages. If any of these appear as non-empty values in config.yml, SetDefaults
// can never apply its code default (RR-007 pitfall).
//
// Key format: dot-separated YAML path (e.g. "service.name").
var setDefaultsOwnedPaths = []string{
	"service.name",
	"database.path",
	"elasticsearch.url",
	"elasticsearch.index",
	"redis.url",
	"redis.channel",
	"observability.log_level",
	"observability.log_format",
}

// TestConfigYMLDoesNotCarrySetDefaultsOwnedValues asserts that config.yml does
// not contain non-empty values for any field owned by SetDefaults (RR-007).
//
// This test FAILS if someone uncomments one of the documented placeholder lines
// in config.yml, which is the desired behaviour — CI will catch the violation.
func TestConfigYMLDoesNotCarrySetDefaultsOwnedValues(t *testing.T) {
	t.Helper()

	data, err := os.ReadFile(configYMLPath)
	require.NoError(t, err, "config.yml must be readable from the alert-crawler module root")

	// Parse into a generic map so we can walk arbitrary paths.
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw), "config.yml must be valid YAML")

	for _, dotPath := range setDefaultsOwnedPaths {
		dotPath := dotPath // capture

		t.Run(dotPath, func(t *testing.T) {
			t.Helper()

			val := walkDotPath(raw, dotPath)
			assert.Nil(t, val,
				"config.yml must not set %q — this field is owned by SetDefaults (RR-007 pitfall). "+
					"Leave it blank or omit it; use an env var for per-environment overrides.",
				dotPath,
			)
		})
	}
}

// walkDotPath traverses a nested map[string]any using a dot-separated key path.
// Returns nil if any segment is missing or if the leaf value is nil / empty string.
func walkDotPath(m map[string]any, dotPath string) any {
	parts := strings.SplitN(dotPath, ".", 2)
	key := parts[0]

	val, ok := m[key]
	if !ok || val == nil {
		return nil
	}

	if len(parts) == 1 {
		// Leaf node: treat empty string as absent (same as omitted).
		if s, isStr := val.(string); isStr && strings.TrimSpace(s) == "" {
			return nil
		}

		return val
	}

	// Recurse into nested map.
	nested, ok := val.(map[string]any)
	if !ok {
		return nil
	}

	return walkDotPath(nested, parts[1])
}

package logs_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestLogEntry_JSON(t *testing.T) {
	t.Helper()

	entry := logs.LogEntry{
		SchemaVersion: logs.CurrentSchemaVersion,
		Timestamp:     time.Date(2026, 1, 28, 12, 0, 0, 0, time.UTC),
		Level:         "info",
		Category:      string(logs.CategoryFetch),
		Message:       "Page fetched",
		JobID:         "job-123",
		ExecID:        "exec-456",
		Fields: map[string]any{
			"url":    "https://example.com",
			"status": 200,
		},
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		t.Fatalf("json.Marshal failed: %v", marshalErr)
	}

	var decoded logs.LogEntry
	unmarshalErr := json.Unmarshal(data, &decoded)
	if unmarshalErr != nil {
		t.Fatalf("json.Unmarshal failed: %v", unmarshalErr)
	}

	if decoded.SchemaVersion != logs.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", decoded.SchemaVersion, logs.CurrentSchemaVersion)
	}
	if decoded.Category != string(logs.CategoryFetch) {
		t.Errorf("Category = %q, want %q", decoded.Category, logs.CategoryFetch)
	}
	if decoded.Message != "Page fetched" {
		t.Errorf("Message = %q, want %q", decoded.Message, "Page fetched")
	}
}

func TestCurrentSchemaVersion(t *testing.T) {
	t.Helper()

	if logs.CurrentSchemaVersion != 1 {
		t.Errorf("CurrentSchemaVersion = %d, want 1", logs.CurrentSchemaVersion)
	}
}

func TestConfig_RedisDefaults(t *testing.T) {
	t.Helper()
	cfg := logs.DefaultConfig()

	if cfg.RedisEnabled {
		t.Error("RedisEnabled should be false by default")
	}
	if cfg.RedisKeyPrefix != "logs" {
		t.Errorf("RedisKeyPrefix = %q, want %q", cfg.RedisKeyPrefix, "logs")
	}
	if cfg.RedisTTLSeconds != 86400 {
		t.Errorf("RedisTTLSeconds = %d, want %d", cfg.RedisTTLSeconds, 86400)
	}
}

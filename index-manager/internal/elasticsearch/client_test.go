package elasticsearch //nolint:testpackage // testing unexported helper functions

import (
	"testing"
)

// --- extractDocumentCount ---

func TestExtractDocumentCount_ValidData(t *testing.T) {
	t.Helper()

	statsData := map[string]any{
		"indices": map[string]any{
			"test_index": map[string]any{
				"total": map[string]any{
					"docs": map[string]any{
						"count": float64(42),
					},
				},
			},
		},
	}

	count := extractDocumentCount(statsData, "test_index")
	if count != 42 {
		t.Errorf("extractDocumentCount() = %d, want 42", count)
	}
}

func TestExtractDocumentCount_MissingIndices(t *testing.T) {
	t.Helper()

	count := extractDocumentCount(map[string]any{}, "test_index")
	if count != 0 {
		t.Errorf("extractDocumentCount() = %d, want 0 for missing indices", count)
	}
}

func TestExtractDocumentCount_MissingIndex(t *testing.T) {
	t.Helper()

	statsData := map[string]any{
		"indices": map[string]any{},
	}

	count := extractDocumentCount(statsData, "nonexistent")
	if count != 0 {
		t.Errorf("extractDocumentCount() = %d, want 0 for missing index", count)
	}
}

func TestExtractDocumentCount_MissingTotal(t *testing.T) {
	t.Helper()

	statsData := map[string]any{
		"indices": map[string]any{
			"test_index": map[string]any{},
		},
	}

	count := extractDocumentCount(statsData, "test_index")
	if count != 0 {
		t.Errorf("extractDocumentCount() = %d, want 0 for missing total", count)
	}
}

func TestExtractDocumentCount_MissingDocs(t *testing.T) {
	t.Helper()

	statsData := map[string]any{
		"indices": map[string]any{
			"test_index": map[string]any{
				"total": map[string]any{},
			},
		},
	}

	count := extractDocumentCount(statsData, "test_index")
	if count != 0 {
		t.Errorf("extractDocumentCount() = %d, want 0 for missing docs", count)
	}
}

func TestExtractDocumentCount_WrongCountType(t *testing.T) {
	t.Helper()

	statsData := map[string]any{
		"indices": map[string]any{
			"test_index": map[string]any{
				"total": map[string]any{
					"docs": map[string]any{
						"count": "not_a_number",
					},
				},
			},
		},
	}

	count := extractDocumentCount(statsData, "test_index")
	if count != 0 {
		t.Errorf("extractDocumentCount() = %d, want 0 for wrong type", count)
	}
}

// --- extractHealthStatus ---

func TestExtractHealthStatus_ValidData(t *testing.T) {
	t.Helper()

	healthData := map[string]any{
		"status": "green",
		"indices": map[string]any{
			"test_index": map[string]any{
				"status": "green",
			},
		},
	}

	health, status := extractHealthStatus(healthData, "test_index")
	if health != "green" {
		t.Errorf("health = %q, want %q", health, "green")
	}
	if status != "green" {
		t.Errorf("status = %q, want %q", status, "green")
	}
}

func TestExtractHealthStatus_MissingClusterStatus(t *testing.T) {
	t.Helper()

	healthData := map[string]any{
		"indices": map[string]any{
			"test_index": map[string]any{
				"status": "yellow",
			},
		},
	}

	health, status := extractHealthStatus(healthData, "test_index")
	if health != unknownStatus {
		t.Errorf("health = %q, want %q for missing cluster status", health, unknownStatus)
	}
	if status != "yellow" {
		t.Errorf("status = %q, want %q", status, "yellow")
	}
}

func TestExtractHealthStatus_MissingIndices(t *testing.T) {
	t.Helper()

	healthData := map[string]any{
		"status": "green",
	}

	health, status := extractHealthStatus(healthData, "test_index")
	if health != "green" {
		t.Errorf("health = %q, want %q", health, "green")
	}
	if status != unknownStatus {
		t.Errorf("status = %q, want %q for missing indices", status, unknownStatus)
	}
}

func TestExtractHealthStatus_MissingIndexEntry(t *testing.T) {
	t.Helper()

	healthData := map[string]any{
		"status":  "green",
		"indices": map[string]any{},
	}

	_, status := extractHealthStatus(healthData, "nonexistent")
	if status != unknownStatus {
		t.Errorf("status = %q, want %q for missing index entry", status, unknownStatus)
	}
}

func TestExtractHealthStatus_MissingIndexStatus(t *testing.T) {
	t.Helper()

	healthData := map[string]any{
		"status": "green",
		"indices": map[string]any{
			"test_index": map[string]any{},
		},
	}

	_, status := extractHealthStatus(healthData, "test_index")
	if status != unknownStatus {
		t.Errorf("status = %q, want %q for missing index status field", status, unknownStatus)
	}
}

func TestExtractHealthStatus_EmptyMap(t *testing.T) {
	t.Helper()

	health, status := extractHealthStatus(map[string]any{}, "test_index")
	if health != unknownStatus {
		t.Errorf("health = %q, want %q for empty map", health, unknownStatus)
	}
	if status != unknownStatus {
		t.Errorf("status = %q, want %q for empty map", status, unknownStatus)
	}
}

// --- Config ---

func TestConfig_Fields(t *testing.T) {
	t.Helper()

	cfg := &Config{
		URL:        "http://localhost:9200",
		Username:   "elastic",
		Password:   "pass",
		MaxRetries: 3,
	}

	if cfg.URL != "http://localhost:9200" {
		t.Errorf("URL = %q, want %q", cfg.URL, "http://localhost:9200")
	}
	if cfg.Username != "elastic" {
		t.Errorf("Username = %q, want %q", cfg.Username, "elastic")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
}

// --- IndexInfo ---

func TestIndexInfo_Fields(t *testing.T) {
	t.Helper()

	info := &IndexInfo{
		Name:          "test_index",
		Health:        "green",
		Status:        "open",
		DocumentCount: 100,
		Size:          "1kb",
	}

	if info.Name != "test_index" {
		t.Errorf("Name = %q, want %q", info.Name, "test_index")
	}
	if info.DocumentCount != 100 {
		t.Errorf("DocumentCount = %d, want 100", info.DocumentCount)
	}
}

// --- IndexDocCount ---

func TestIndexDocCount_Fields(t *testing.T) {
	t.Helper()

	idc := IndexDocCount{Name: "my_index", DocCount: 500}
	if idc.Name != "my_index" {
		t.Errorf("Name = %q, want %q", idc.Name, "my_index")
	}
	if idc.DocCount != 500 {
		t.Errorf("DocCount = %d, want 500", idc.DocCount)
	}
}

// --- GetClient ---

func TestClient_GetClient_Nil(t *testing.T) {
	t.Helper()

	c := &Client{}
	if c.GetClient() != nil {
		t.Error("GetClient() should return nil for zero-value Client")
	}
}

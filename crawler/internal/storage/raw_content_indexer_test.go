package storage_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// mockIndexManager records the mapping passed to EnsureIndex for assertions.
type mockIndexManager struct {
	lastEnsureIndexName    string
	lastEnsureIndexMapping any
	indexExists            bool
}

func (m *mockIndexManager) EnsureIndex(_ context.Context, name string, mapping any) error {
	m.lastEnsureIndexName = name
	m.lastEnsureIndexMapping = mapping
	return nil
}

func (m *mockIndexManager) DeleteIndex(context.Context, string) error { return nil }
func (m *mockIndexManager) IndexExists(_ context.Context, _ string) (bool, error) {
	return m.indexExists, nil
}
func (m *mockIndexManager) GetMapping(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (m *mockIndexManager) UpdateMapping(context.Context, string, map[string]any) error {
	return nil
}

// mockStorage implements types.Interface and delegates index operations to mockIndexManager.
type mockStorageWithIndexManager struct {
	indexManager *mockIndexManager
}

func (m *mockStorageWithIndexManager) GetIndexManager() types.IndexManager {
	return m.indexManager
}

func (m *mockStorageWithIndexManager) IndexDocument(context.Context, string, string, any) error {
	return nil
}
func (m *mockStorageWithIndexManager) GetDocument(context.Context, string, string, any) error {
	return nil
}
func (m *mockStorageWithIndexManager) DeleteDocument(context.Context, string, string) error {
	return nil
}
func (m *mockStorageWithIndexManager) CreateIndex(context.Context, string, map[string]any) error {
	return nil
}
func (m *mockStorageWithIndexManager) DeleteIndex(context.Context, string) error {
	return nil
}
func (m *mockStorageWithIndexManager) IndexExists(context.Context, string) (bool, error) {
	return false, nil
}
func (m *mockStorageWithIndexManager) ListIndices(context.Context) ([]string, error) {
	return []string{}, nil
}
func (m *mockStorageWithIndexManager) GetMapping(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (m *mockStorageWithIndexManager) UpdateMapping(context.Context, string, map[string]any) error {
	return nil
}
func (m *mockStorageWithIndexManager) GetIndexHealth(context.Context, string) (string, error) {
	return "", nil
}
func (m *mockStorageWithIndexManager) GetIndexDocCount(context.Context, string) (int64, error) {
	return 0, nil
}
func (m *mockStorageWithIndexManager) TestConnection(context.Context) error {
	return nil
}
func (m *mockStorageWithIndexManager) Close() error {
	return nil
}

func TestEnsureRawContentIndex_UsesCanonicalMapping(t *testing.T) {
	t.Helper()

	mockIM := &mockIndexManager{indexExists: false}
	mockStorage := &mockStorageWithIndexManager{indexManager: mockIM}
	logger := infralogger.NewNop()
	indexer := storage.NewRawContentIndexer(mockStorage, logger)

	ctx := context.Background()
	err := indexer.EnsureRawContentIndex(ctx, "example.com")
	if err != nil {
		t.Fatalf("EnsureRawContentIndex: %v", err)
	}

	if mockIM.lastEnsureIndexName != "example_com_raw_content" {
		t.Errorf("expected index name example_com_raw_content, got %q", mockIM.lastEnsureIndexName)
	}

	mapping, ok := mockIM.lastEnsureIndexMapping.(map[string]any)
	if !ok {
		t.Fatalf("expected mapping to be map[string]any, got %T", mockIM.lastEnsureIndexMapping)
	}

	mappingsObj, ok := mapping["mappings"].(map[string]any)
	if !ok {
		t.Fatal("mapping missing mappings key or not map")
	}
	properties, ok := mappingsObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("mappings.properties missing or not map")
	}
	jsonLDData, ok := properties["json_ld_data"].(map[string]any)
	if !ok {
		t.Fatal("mappings.properties.json_ld_data missing or not map")
	}
	jsonLDProps, ok := jsonLDData["properties"].(map[string]any)
	if !ok {
		t.Fatal("json_ld_data.properties missing or not map")
	}
	jsonldRaw, ok := jsonLDProps["jsonld_raw"].(map[string]any)
	if !ok {
		t.Fatal("json_ld_data.properties.jsonld_raw missing or not map")
	}
	enabled, ok := jsonldRaw["enabled"].(bool)
	if !ok {
		t.Fatal("jsonld_raw.enabled missing or not bool")
	}
	if enabled {
		t.Error("expected jsonld_raw.enabled to be false (canonical mapping avoids dynamic mapping conflicts)")
	}
}

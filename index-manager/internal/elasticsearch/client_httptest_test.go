package elasticsearch //nolint:testpackage // testing Client methods with httptest mock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	es "github.com/elastic/go-elasticsearch/v8"
)

// esProductHandler wraps a handler to add the X-Elastic-Product header
// required by the go-elasticsearch v8 client for product validation.
func esProductHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		h.ServeHTTP(w, r)
	})
}

// newTestClient creates an ES Client backed by an httptest server.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(esProductHandler(handler))
	t.Cleanup(srv.Close)

	esCfg := es.Config{
		Addresses: []string{srv.URL},
	}
	esClient, err := es.NewClient(esCfg)
	if err != nil {
		t.Fatalf("failed to create ES client: %v", err)
	}

	return &Client{
		esClient: esClient,
		config:   &Config{URL: srv.URL},
	}
}

// --- IndexExists ---

func TestIndexExists_True(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client := newTestClient(t, handler)

	exists, err := client.IndexExists(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected index to exist")
	}
}

func TestIndexExists_False(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	exists, err := client.IndexExists(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected index to not exist")
	}
}

// --- CreateIndex ---

func TestCreateIndex_Success(t *testing.T) {
	t.Helper()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// IndexExists check
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Create index
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	})
	client := newTestClient(t, handler)

	err := client.CreateIndex(context.Background(), "new_index", map[string]any{"mappings": map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateIndex_AlreadyExists(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client := newTestClient(t, handler)

	err := client.CreateIndex(context.Background(), "existing_index", nil)
	if err == nil {
		t.Fatal("expected error for existing index")
	}
}

func TestCreateIndex_NilMapping(t *testing.T) {
	t.Helper()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	})
	client := newTestClient(t, handler)

	err := client.CreateIndex(context.Background(), "new_index", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateIndex_ESError(t *testing.T) {
	t.Helper()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	})
	client := newTestClient(t, handler)

	err := client.CreateIndex(context.Background(), "bad_index", nil)
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- EnsureIndex ---

func TestEnsureIndex_AlreadyExists(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	client := newTestClient(t, handler)

	err := client.EnsureIndex(context.Background(), "existing_index", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureIndex_CreatesNew(t *testing.T) {
	t.Helper()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		switch {
		case callCount <= 2:
			// First two calls: IndexExists from EnsureIndex and CreateIndex
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		}
	})
	client := newTestClient(t, handler)

	err := client.EnsureIndex(context.Background(), "new_index", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- DeleteIndex ---

func TestDeleteIndex_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	})
	client := newTestClient(t, handler)

	err := client.DeleteIndex(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteIndex_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"index_not_found"}`))
	})
	client := newTestClient(t, handler)

	err := client.DeleteIndex(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- ListIndices ---

func TestListIndices_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := []map[string]any{
			{"index": "test_raw_content"},
			{"index": "test_classified_content"},
			{"index": ".kibana"}, // system index, should be filtered
		}
		w.WriteHeader(http.StatusOK)
		//nolint:errchkjson // test mock handler
		_ = json.NewEncoder(w).Encode(resp)
	})
	client := newTestClient(t, handler)

	indices, err := client.ListIndices(context.Background(), "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 2 {
		t.Errorf("indices count = %d, want 2 (system index filtered)", len(indices))
	}
}

func TestListIndices_EmptyPattern(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})
	client := newTestClient(t, handler)

	indices, err := client.ListIndices(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 0 {
		t.Errorf("indices count = %d, want 0", len(indices))
	}
}

func TestListIndices_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.ListIndices(context.Background(), "*")
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- GetAllIndexDocCounts ---

func TestGetAllIndexDocCounts_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := []map[string]string{
			{"index": "test_raw_content", "docs.count": "100"},
			{"index": "test_classified_content", "docs.count": "80"},
			{"index": ".kibana", "docs.count": "5"},
		}
		w.WriteHeader(http.StatusOK)
		//nolint:errchkjson // test mock handler
		_ = json.NewEncoder(w).Encode(resp)
	})
	client := newTestClient(t, handler)

	counts, err := client.GetAllIndexDocCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("counts = %d, want 2 (system index filtered)", len(counts))
	}
	if counts[0].DocCount != 100 {
		t.Errorf("first doc count = %d, want 100", counts[0].DocCount)
	}
}

func TestGetAllIndexDocCounts_InvalidDocCount(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := []map[string]string{
			{"index": "test_index", "docs.count": "not_a_number"},
		}
		w.WriteHeader(http.StatusOK)
		//nolint:errchkjson // test mock handler
		_ = json.NewEncoder(w).Encode(resp)
	})
	client := newTestClient(t, handler)

	counts, err := client.GetAllIndexDocCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 1 {
		t.Fatalf("counts = %d, want 1", len(counts))
	}
	if counts[0].DocCount != 0 {
		t.Errorf("invalid doc count should be 0, got %d", counts[0].DocCount)
	}
}

// --- GetIndexHealth ---

func TestGetIndexHealth_Green(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"green"}`))
	})
	client := newTestClient(t, handler)

	health, err := client.GetIndexHealth(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health != "green" {
		t.Errorf("health = %q, want %q", health, "green")
	}
}

func TestGetIndexHealth_NoStatus(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	client := newTestClient(t, handler)

	health, err := client.GetIndexHealth(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health != unknownStatus {
		t.Errorf("health = %q, want %q", health, unknownStatus)
	}
}

func TestGetIndexHealth_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.GetIndexHealth(context.Background(), "test_index")
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- GetClusterHealth ---

func TestGetClusterHealth_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"green","number_of_nodes":1}`))
	})
	client := newTestClient(t, handler)

	health, err := client.GetClusterHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health["status"] != "green" {
		t.Errorf("status = %v, want %q", health["status"], "green")
	}
}

func TestGetClusterHealth_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"cluster unavailable"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.GetClusterHealth(context.Background())
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- GetDocument ---

func TestGetDocument_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"_source":{"title":"Test","url":"https://example.com"}}`))
	})
	client := newTestClient(t, handler)

	doc, err := client.GetDocument(context.Background(), "test_index", "doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc["title"] != "Test" {
		t.Errorf("title = %v, want %q", doc["title"], "Test")
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"found":false}`))
	})
	client := newTestClient(t, handler)

	_, err := client.GetDocument(context.Background(), "test_index", "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found document")
	}
}

// --- DeleteDocument ---

func TestDeleteDocument_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"deleted"}`))
	})
	client := newTestClient(t, handler)

	err := client.DeleteDocument(context.Background(), "test_index", "doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"found":false}`))
	})
	client := newTestClient(t, handler)

	err := client.DeleteDocument(context.Background(), "test_index", "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found document")
	}
}

// --- UpdateDocument ---

func TestUpdateDocument_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	})
	client := newTestClient(t, handler)

	err := client.UpdateDocument(context.Background(), "test_index", "doc-1", map[string]any{"title": "Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateDocument_NotFound(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"document_missing"}`))
	})
	client := newTestClient(t, handler)

	err := client.UpdateDocument(context.Background(), "test_index", "nonexistent", map[string]any{"title": "X"})
	if err == nil {
		t.Fatal("expected error for not found document")
	}
}

// --- SearchDocuments ---

func TestSearchDocuments_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_id":"1","_source":{"title":"Test"}}]}}`))
	})
	client := newTestClient(t, handler)

	res, err := client.SearchDocuments(context.Background(), "test_index", map[string]any{"query": map[string]any{"match_all": map[string]any{}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
}

func TestSearchDocuments_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad query"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.SearchDocuments(context.Background(), "test_index", map[string]any{})
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- BulkDeleteDocuments ---

func TestBulkDeleteDocuments_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"delete":{"_id":"1","status":200}},{"delete":{"_id":"2","status":200}}]}`))
	})
	client := newTestClient(t, handler)

	err := client.BulkDeleteDocuments(context.Background(), "test_index", []string{"1", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBulkDeleteDocuments_EmptyIDs(t *testing.T) {
	t.Helper()

	client := &Client{}
	err := client.BulkDeleteDocuments(context.Background(), "test_index", []string{})
	if err == nil {
		t.Fatal("expected error for empty document IDs")
	}
}

func TestBulkDeleteDocuments_WithErrors(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"delete":{"_id":"1","error":{"type":"not_found","reason":"doc missing"}}}]}`))
	})
	client := newTestClient(t, handler)

	err := client.BulkDeleteDocuments(context.Background(), "test_index", []string{"1"})
	if err == nil {
		t.Fatal("expected error for bulk delete with individual errors")
	}
}

// --- Reindex ---

func TestReindex_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":500}`))
	})
	client := newTestClient(t, handler)

	total, err := client.Reindex(context.Background(), "source_index", "dest_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 500 {
		t.Errorf("total = %d, want 500", total)
	}
}

func TestReindex_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"reindex failed"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.Reindex(context.Background(), "source", "dest")
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- GetIndexMapping ---

func TestGetIndexMapping_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"test_index": map[string]any{
				"mappings": map[string]any{
					"properties": map[string]any{
						"title": map[string]any{"type": "text"},
					},
				},
			},
		}
		//nolint:errchkjson // test mock handler
		_ = json.NewEncoder(w).Encode(resp)
	})
	client := newTestClient(t, handler)

	mapping, err := client.GetIndexMapping(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping == nil {
		t.Fatal("mapping should not be nil")
	}
}

func TestGetIndexMapping_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"index_not_found"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.GetIndexMapping(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

func TestGetIndexMapping_MissingMappings(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"test_index": map[string]any{},
		}
		//nolint:errchkjson // test mock handler
		_ = json.NewEncoder(w).Encode(resp)
	})
	client := newTestClient(t, handler)

	_, err := client.GetIndexMapping(context.Background(), "test_index")
	if err == nil {
		t.Fatal("expected error for missing mappings")
	}
}

// --- UpdateIndexMapping ---

func TestUpdateIndexMapping_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	})
	client := newTestClient(t, handler)

	err := client.UpdateIndexMapping(context.Background(), "test_index", map[string]any{
		"properties": map[string]any{"new_field": map[string]any{"type": "keyword"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateIndexMapping_ESError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"mapping update failed"}`))
	})
	client := newTestClient(t, handler)

	err := client.UpdateIndexMapping(context.Background(), "test_index", map[string]any{})
	if err == nil {
		t.Fatal("expected error for ES error response")
	}
}

// --- GetIndexInfo ---

func TestGetIndexInfo_Success(t *testing.T) {
	t.Helper()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)

		switch callCount {
		case 1:
			// Stats response
			resp := map[string]any{
				"indices": map[string]any{
					"test_index": map[string]any{
						"total": map[string]any{
							"docs": map[string]any{"count": float64(42)},
						},
					},
				},
			}
			//nolint:errchkjson // test mock handler
			_ = json.NewEncoder(w).Encode(resp)
		case 2:
			// Health response
			resp := map[string]any{
				"status": "green",
				"indices": map[string]any{
					"test_index": map[string]any{"status": "open"},
				},
			}
			//nolint:errchkjson // test mock handler
			_ = json.NewEncoder(w).Encode(resp)
		default:
			// Index info response
			resp := map[string]any{
				"test_index": map[string]any{
					"settings": map[string]any{"number_of_shards": "1"},
					"mappings": map[string]any{"properties": map[string]any{}},
				},
			}
			//nolint:errchkjson // test mock handler
			_ = json.NewEncoder(w).Encode(resp)
		}
	})
	client := newTestClient(t, handler)

	info, err := client.GetIndexInfo(context.Background(), "test_index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "test_index" {
		t.Errorf("Name = %q, want %q", info.Name, "test_index")
	}
	if info.DocumentCount != 42 {
		t.Errorf("DocumentCount = %d, want 42", info.DocumentCount)
	}
	if info.Health != "green" {
		t.Errorf("Health = %q, want %q", info.Health, "green")
	}
}

func TestGetIndexInfo_StatsError(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"stats failed"}`))
	})
	client := newTestClient(t, handler)

	_, err := client.GetIndexInfo(context.Background(), "test_index")
	if err == nil {
		t.Fatal("expected error for stats failure")
	}
}

// --- SearchAllClassifiedContent ---

func TestSearchAllClassifiedContent_Success(t *testing.T) {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"hits":{"total":{"value":10},"hits":[]}}`)
	})
	client := newTestClient(t, handler)

	res, err := client.SearchAllClassifiedContent(context.Background(), map[string]any{"size": 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
}

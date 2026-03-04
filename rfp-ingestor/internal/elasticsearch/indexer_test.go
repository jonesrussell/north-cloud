package elasticsearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

func TestBulkIndex_SendsDocuments(t *testing.T) {
	var capturedBody string
	var capturedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_bulk":
			capturedContentType = r.Header.Get("Content-Type")

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			capturedBody = string(body)

			// Respond with a successful bulk response for 1 item.
			resp := bulkResponse{
				Errors: false,
				Items: []bulkResponseItem{
					{Index: struct {
						Status int              `json:"status"`
						Error  *json.RawMessage `json:"error,omitempty"`
					}{Status: 201}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			// ES root ping or other paths.
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	indexer, err := NewIndexer(srv.URL, "test-index", 100)
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	doc := domain.RFPDocument{
		Title:      "Test RFP",
		URL:        "https://example.com/rfp/1",
		SourceName: "TestSource",
		RFP: domain.RFP{
			ReferenceNumber: "REF-001",
			Title:           "Test RFP",
		},
	}
	docs := map[string]domain.RFPDocument{
		"doc-id-abc": doc,
	}

	result, err := indexer.BulkIndex(context.Background(), docs)
	if err != nil {
		t.Fatalf("BulkIndex: %v", err)
	}

	if result.Indexed != 1 {
		t.Errorf("Indexed: expected 1, got %d", result.Indexed)
	}
	if result.Failed != 0 {
		t.Errorf("Failed: expected 0, got %d", result.Failed)
	}

	// Verify content type.
	if capturedContentType != "application/x-ndjson" {
		t.Errorf("Content-Type: expected application/x-ndjson, got %q", capturedContentType)
	}

	// Verify NDJSON body structure: action line then document line.
	lines := strings.Split(strings.TrimSpace(capturedBody), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %v", len(lines), lines)
	}

	// Action line should contain the correct _id and _index.
	var action bulkAction
	if err := json.Unmarshal([]byte(lines[0]), &action); err != nil {
		t.Fatalf("unmarshal action line: %v", err)
	}
	if action.Index.ID != "doc-id-abc" {
		t.Errorf("action _id: expected %q, got %q", "doc-id-abc", action.Index.ID)
	}
	if action.Index.Index != "test-index" {
		t.Errorf("action _index: expected %q, got %q", "test-index", action.Index.Index)
	}

	// Document line should contain the RFP document.
	var parsedDoc domain.RFPDocument
	if err := json.Unmarshal([]byte(lines[1]), &parsedDoc); err != nil {
		t.Fatalf("unmarshal document line: %v", err)
	}
	if parsedDoc.Title != "Test RFP" {
		t.Errorf("document title: expected %q, got %q", "Test RFP", parsedDoc.Title)
	}
	if parsedDoc.RFP.ReferenceNumber != "REF-001" {
		t.Errorf("document reference_number: expected %q, got %q", "REF-001", parsedDoc.RFP.ReferenceNumber)
	}
}

func TestBulkIndex_EmptyBatch(t *testing.T) {
	// A bogus URL is fine here because no HTTP call should be made.
	indexer, err := NewIndexer("http://bogus:9999", "test-index", 100)
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}

	result, err := indexer.BulkIndex(context.Background(), map[string]domain.RFPDocument{})
	if err != nil {
		t.Fatalf("BulkIndex with empty map: %v", err)
	}

	if result.Indexed != 0 {
		t.Errorf("Indexed: expected 0, got %d", result.Indexed)
	}
	if result.Failed != 0 {
		t.Errorf("Failed: expected 0, got %d", result.Failed)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors: expected none, got %v", result.Errors)
	}
}

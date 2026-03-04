package ingestor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/north-cloud/infrastructure/logger"
)

func TestIngestor_RunOnce(t *testing.T) {
	// Build a CSV payload with the full 67-column header and one Open IT row.
	row := makeRow(map[int]string{
		idxTitle:          "IT Software Modernization Services",
		idxRefNumber:      "PW-24-01234567",
		idxAmendment:      "000",
		idxPubDate:        "2024-11-15",
		idxClosingDate:    "2025-01-15",
		idxAmendmentDate:  "2024-12-01",
		idxStatusEng:      "Open",
		idxGSIN:           "*D121",
		idxUNSPSC:         "*43232300",
		idxProcurementCat: "SV",
		idxRegionDelivery: "Ontario",
		idxOrgName:        "Shared Services Canada",
		idxCity:           "Ottawa",
		idxContactName:    "Jane Doe",
		idxContactEmail:   "jane.doe@canada.ca",
		idxNoticeURL:      "https://canadabuys.canada.ca/en/tender/PW-24-01234567",
		idxDescriptionEng: "Modernization of legacy IT systems.",
	})
	csvPayload := fullHeader + "\n" + row + "\n"

	// Mock CSV feed server.
	csvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(csvPayload))
	}))
	defer csvServer.Close()

	// Mock Elasticsearch server handling _bulk requests.
	esServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/_bulk":
			resp := map[string]any{
				"errors": false,
				"items": []map[string]any{
					{"index": map[string]any{"status": 201}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer esServer.Close()

	cfg := Config{
		FeedURL:  csvServer.URL,
		ESURL:    esServer.URL,
		ESIndex:  "test-rfp-index",
		BulkSize: 100,
	}

	ing := NewIngestor(cfg, logger.NewNop())
	result, err := ing.RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.Fetched != 1 {
		t.Errorf("Fetched: expected 1, got %d", result.Fetched)
	}
	if result.Indexed != 1 {
		t.Errorf("Indexed: expected 1, got %d", result.Indexed)
	}
	if result.Failed != 0 {
		t.Errorf("Failed: expected 0, got %d", result.Failed)
	}
	if result.Duration <= 0 {
		t.Errorf("Duration should be positive, got %v", result.Duration)
	}
}

func TestIngestor_RunOnce_NotModified(t *testing.T) {
	// Server always returns 304 Not Modified.
	csvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer csvServer.Close()

	cfg := Config{
		FeedURL:  csvServer.URL,
		ESURL:    "http://unused:9200",
		ESIndex:  "test-rfp-index",
		BulkSize: 100,
	}

	// Pre-seed the fetcher's lastModified so it sends If-Modified-Since and
	// our server responds with 304.
	ing := NewIngestor(cfg, logger.NewNop())

	result, err := ing.RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if result.Fetched != 0 {
		t.Errorf("Fetched: expected 0 for 304, got %d", result.Fetched)
	}
	if result.Indexed != 0 {
		t.Errorf("Indexed: expected 0 for 304, got %d", result.Indexed)
	}
}

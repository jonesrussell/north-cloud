package elasticsearch_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
)

// newTestIndexer builds an Indexer pointed at the given httptest server.
func newTestIndexer(t *testing.T, srv *httptest.Server) *elasticsearch.Indexer {
	t.Helper()

	return elasticsearch.New(elasticsearch.Config{
		BaseURL: srv.URL,
		Index:   "community_alerts",
	})
}

// minAlert returns the smallest valid domain.Alert usable in tests.
// Hazard.HarmReduction is populated to satisfy MarshalJSON (v1 requirement).
func minAlert(id string) domain.Alert {
	ts := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	return domain.Alert{
		ID:             id,
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityHigh,
		Scope:          []string{"Vancouver"},
		IssuedAt:       ts,
		LifecycleState: domain.LifecycleActive,
		Title:          "Test Alert",
		Summary:        "Test summary.",
		Hazard: domain.Hazard{
			HarmReduction: &domain.HarmReductionHazard{
				HazardType: domain.HazardOpioidSupply,
				Substances: []string{"fentanyl"},
			},
		},
		Sources: []domain.SourceAttribution{
			{SourceID: "src-1", SourceName: "SaferSites", URL: "https://example.com"},
		},
		ParseQuality:  domain.ParseClean,
		CrawledAt:     ts,
		LastUpdatedAt: ts,
	}
}

// TestEnsureIndex_NotFoundCreates verifies that when HEAD returns 404,
// the indexer issues a PUT request whose body matches the embedded mapping.
func TestEnsureIndex_NotFoundCreates(t *testing.T) {
	t.Helper()

	putCalled := false
	var putBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			putCalled = true
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("read PUT body: %v", readErr)
			}

			putBody = body
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	if !putCalled {
		t.Fatal("expected PUT to be called after 404 HEAD")
	}

	// Verify the PUT body parses as valid JSON and matches embedded mapping.
	var got map[string]any
	if err := json.Unmarshal(putBody, &got); err != nil {
		t.Fatalf("PUT body not valid JSON: %v", err)
	}

	var want map[string]any
	if err := json.Unmarshal(elasticsearch.CommunityAlertsMapping(), &want); err != nil {
		t.Fatalf("embedded mapping not valid JSON: %v", err)
	}

	gotJSON, marshalGotErr := json.Marshal(got)
	if marshalGotErr != nil {
		t.Fatalf("re-marshal received body: %v", marshalGotErr)
	}

	wantJSON, marshalWantErr := json.Marshal(want)
	if marshalWantErr != nil {
		t.Fatalf("re-marshal embedded mapping: %v", marshalWantErr)
	}

	if !bytes.Equal(gotJSON, wantJSON) {
		t.Errorf("PUT body mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

// TestEnsureIndex_ExistsNoOp verifies that when HEAD returns 200, no PUT is issued.
func TestEnsureIndex_ExistsNoOp(t *testing.T) {
	t.Helper()

	putCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	if putCalled {
		t.Error("expected no PUT when index already exists (HEAD 200)")
	}
}

// TestEnsureIndex_RaceCondition verifies that a 400 + resource_already_exists_exception
// on PUT is treated as success (concurrent EnsureIndex callers).
func TestEnsureIndex_RaceCondition(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"resource_already_exists_exception","reason":"already exists"}}`))
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("EnsureIndex should treat resource_already_exists_exception as success, got: %v", err)
	}
}

// TestIndex_RoundTrip verifies that Index issues a PUT /_doc/{id} with the marshaled alert.
func TestIndex_RoundTrip(t *testing.T) {
	t.Helper()

	alert := minAlert("alert-abc-123")

	var capturedBody []byte
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "expected PUT", http.StatusMethodNotAllowed)
			return
		}

		capturedPath = r.URL.Path

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body: %v", readErr)
		}

		capturedBody = body
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result":"created"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.Index(context.Background(), alert); err != nil {
		t.Fatalf("Index: %v", err)
	}

	expectedPath := "/community_alerts/_doc/" + alert.ID
	if capturedPath != expectedPath {
		t.Errorf("path = %q, want %q", capturedPath, expectedPath)
	}

	expected, marshalExpErr := json.Marshal(alert)
	if marshalExpErr != nil {
		t.Fatalf("marshal expected alert: %v", marshalExpErr)
	}

	if !bytes.Equal(capturedBody, expected) {
		t.Errorf("body mismatch\ngot:  %s\nwant: %s", capturedBody, expected)
	}
}

// TestMarkRescinded_PartialUpdate verifies that MarkRescinded issues a POST /_update/{id}
// with a Painless script payload containing lifecycle_state=rescinded and revision_history append.
func TestMarkRescinded_PartialUpdate(t *testing.T) {
	t.Helper()

	alertID := "alert-xyz-999"
	rescindTime := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	reason := "Alert withdrawn by source"

	var capturedPath string
	var capturedBody []byte
	var capturedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}

		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body: %v", readErr)
		}

		capturedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"updated"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.MarkRescinded(context.Background(), alertID, rescindTime, reason); err != nil {
		t.Fatalf("MarkRescinded: %v", err)
	}

	expectedPath := "/community_alerts/_update/" + alertID
	if capturedPath != expectedPath {
		t.Errorf("path = %q, want %q", capturedPath, expectedPath)
	}

	if capturedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", capturedContentType)
	}

	// Validate payload structure.
	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}

	script, ok := payload["script"].(map[string]any)
	if !ok {
		t.Fatal("missing 'script' key in payload")
	}

	params, ok := script["params"].(map[string]any)
	if !ok {
		t.Fatal("missing 'params' in script")
	}

	if params["lifecycle_state"] != "rescinded" {
		t.Errorf("params.lifecycle_state = %v, want rescinded", params["lifecycle_state"])
	}

	revEntry, ok := params["revision_entry"].(map[string]any)
	if !ok {
		t.Fatal("missing 'revision_entry' in params")
	}

	if revEntry["revision_kind"] != "rescinded" {
		t.Errorf("revision_entry.revision_kind = %v, want rescinded", revEntry["revision_kind"])
	}

	if revEntry["change_summary"] != reason {
		t.Errorf("revision_entry.change_summary = %v, want %q", revEntry["change_summary"], reason)
	}
}

// TestQueryActive_BuildsQuery verifies the search request shape for an active-alert query.
func TestQueryActive_BuildsQuery(t *testing.T) {
	t.Helper()

	var capturedBody []byte
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body: %v", readErr)
		}

		capturedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	alerts, err := ix.QueryActive(context.Background(), "src-test")
	if err != nil {
		t.Fatalf("QueryActive: %v", err)
	}

	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}

	if capturedPath != "/community_alerts/_search" {
		t.Errorf("path = %q, want /community_alerts/_search", capturedPath)
	}

	// Validate query structure.
	var q map[string]any
	if unmarshalErr := json.Unmarshal(capturedBody, &q); unmarshalErr != nil {
		t.Fatalf("query not valid JSON: %v", unmarshalErr)
	}

	query, ok := q["query"].(map[string]any)
	if !ok {
		t.Fatal("missing 'query' key")
	}

	boolClause, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatal("missing 'query.bool' key")
	}

	must, ok := boolClause["must"].([]any)
	if !ok {
		t.Fatal("missing 'query.bool.must'")
	}

	// Must have at least the lifecycle_state term and the nested sources filter.
	const minMustClauses = 2 // lifecycle term + nested source filter
	if len(must) < minMustClauses {
		t.Errorf("must clauses count = %d, want >= %d", len(must), minMustClauses)
	}
}

// TestQueryActiveAlertIDs_NoSource verifies the ESActiveAlertQuerier interface method
// (no sourceID param) returns all active alerts without a nested source filter.
func TestQueryActiveAlertIDs_NoSource(t *testing.T) {
	t.Helper()

	alert := minAlert("alert-querier-001")

	alertJSON, marshalErr := json.Marshal(alert)
	if marshalErr != nil {
		t.Fatalf("marshal test alert: %v", marshalErr)
	}

	// Build the ES _search response body.
	type hitSource struct {
		Source json.RawMessage `json:"_source"`
	}
	type hitsInner struct {
		Hits []hitSource `json:"hits"`
	}
	type searchResp struct {
		Hits hitsInner `json:"hits"`
	}

	respPayload := searchResp{
		Hits: hitsInner{
			Hits: []hitSource{{Source: json.RawMessage(alertJSON)}},
		},
	}

	respBody, encErr := json.Marshal(respPayload)
	if encErr != nil {
		t.Fatalf("encode test response: %v", encErr)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body: %v", readErr)
		}

		// No nested source filter when sourceID is empty.
		var q map[string]any
		if jsonErr := json.Unmarshal(body, &q); jsonErr != nil {
			t.Errorf("invalid query JSON: %v", jsonErr)
		}

		boolMust := q["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
		if len(boolMust) != 1 {
			t.Errorf("expected exactly 1 must clause (no source filter), got %d", len(boolMust))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBody)
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	alerts, err := ix.QueryActiveAlertIDs(context.Background())
	if err != nil {
		t.Fatalf("QueryActiveAlertIDs: %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	if alerts[0].ID != alert.ID {
		t.Errorf("alert ID = %q, want %q", alerts[0].ID, alert.ID)
	}
}

// TestBulkIndex_NDJSON verifies the _bulk request body is correct NDJSON.
func TestBulkIndex_NDJSON(t *testing.T) {
	t.Helper()

	alerts := []domain.Alert{
		minAlert("bulk-1"),
		minAlert("bulk-2"),
	}

	var capturedBody []byte
	var capturedContentType string
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body: %v", readErr)
		}

		capturedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":false,"items":[]}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.BulkIndex(context.Background(), alerts); err != nil {
		t.Fatalf("BulkIndex: %v", err)
	}

	if capturedPath != "/_bulk" {
		t.Errorf("path = %q, want /_bulk", capturedPath)
	}

	if capturedContentType != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", capturedContentType)
	}

	// NDJSON: each pair of lines = action + document.
	const linesPerAlert = 2 // action line + document line
	lines := strings.Split(strings.TrimRight(string(capturedBody), "\n"), "\n")
	expectedLines := len(alerts) * linesPerAlert
	if len(lines) != expectedLines {
		t.Errorf("NDJSON line count = %d, want %d", len(lines), expectedLines)
	}

	// Verify action lines.
	for i, alert := range alerts {
		actionLine := lines[i*linesPerAlert]
		var action map[string]any
		if err := json.Unmarshal([]byte(actionLine), &action); err != nil {
			t.Errorf("action line %d not valid JSON: %v", i, err)
			continue
		}

		indexAction, ok := action["index"].(map[string]any)
		if !ok {
			t.Errorf("action line %d missing 'index' key", i)
			continue
		}

		if indexAction["_id"] != alert.ID {
			t.Errorf("action[%d]._id = %v, want %q", i, indexAction["_id"], alert.ID)
		}
	}
}

// TestBulkIndex_Empty verifies BulkIndex with an empty slice is a no-op (no HTTP call).
func TestBulkIndex_Empty(t *testing.T) {
	t.Helper()

	called := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.BulkIndex(context.Background(), nil); err != nil {
		t.Fatalf("BulkIndex empty: %v", err)
	}

	if called {
		t.Error("expected no HTTP call for empty BulkIndex")
	}
}

// TestEnsureIndex_UnexpectedStatus verifies that a non-200/404 HEAD response returns an error.
func TestEnsureIndex_UnexpectedStatus(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.EnsureIndex(context.Background()); err == nil {
		t.Error("expected error for unexpected HEAD status, got nil")
	}
}

// TestEnsureIndex_PutMappingError verifies that a non-race-condition 400 from PUT returns an error.
func TestEnsureIndex_PutMappingError(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"mapper_parsing_exception","reason":"bad mapping"}}`))
		default:
			http.Error(w, "unexpected", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.EnsureIndex(context.Background()); err == nil {
		t.Error("expected error for non-race-condition PUT 400, got nil")
	}
}

// TestIndex_ErrorResponse verifies Index surfaces a 4xx error from ES.
func TestIndex_ErrorResponse(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"version conflict"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.Index(context.Background(), minAlert("err-alert")); err == nil {
		t.Error("expected error for 409 response, got nil")
	}
}

// TestMarkRescinded_ErrorResponse verifies MarkRescinded surfaces a 4xx error.
func TestMarkRescinded_ErrorResponse(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"document missing"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	at := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := ix.MarkRescinded(context.Background(), "missing-id", at, "gone"); err == nil {
		t.Error("expected error for 404 MarkRescinded, got nil")
	}
}

// TestQueryActive_ErrorResponse verifies QueryActive surfaces a 4xx error.
func TestQueryActive_ErrorResponse(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad query"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if _, err := ix.QueryActive(context.Background(), ""); err == nil {
		t.Error("expected error for 400 search response, got nil")
	}
}

// TestBulkIndex_ErrorResponse verifies BulkIndex surfaces a 4xx error from ES.
func TestBulkIndex_ErrorResponse(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"error":"payload too large"}`))
	}))
	defer srv.Close()

	ix := newTestIndexer(t, srv)

	if err := ix.BulkIndex(context.Background(), []domain.Alert{minAlert("bulk-err")}); err == nil {
		t.Error("expected error for 413 BulkIndex response, got nil")
	}
}

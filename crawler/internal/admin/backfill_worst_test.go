package admin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/admin"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// mockESSearcher implements admin.ESSearcher for testing.
type mockESSearcher struct {
	indexStats  map[string]mockIndexStats
	indexExists map[string]bool
}

type mockIndexStats struct {
	avgWordCount float64
	docCount     int64
}

func newMockESSearcher() *mockESSearcher {
	return &mockESSearcher{
		indexStats:  make(map[string]mockIndexStats),
		indexExists: make(map[string]bool),
	}
}

func (m *mockESSearcher) IndexExists(_ context.Context, index string) (bool, error) {
	exists, ok := m.indexExists[index]
	if !ok {
		return false, nil
	}
	return exists, nil
}

func (m *mockESSearcher) SearchDocuments(
	_ context.Context,
	index string,
	query map[string]any,
	result any,
) error {
	stats, ok := m.indexStats[index]
	if !ok {
		return fmt.Errorf("index not found: %s", index)
	}

	// Build a mock ES response based on whether this is an agg or count query
	var response map[string]any

	if _, hasAggs := query["aggs"]; hasAggs {
		response = map[string]any{
			"hits": map[string]any{
				"total": map[string]any{
					"value": float64(stats.docCount),
				},
			},
			"aggregations": map[string]any{
				"avg_word_count": map[string]any{
					"value": stats.avgWordCount,
				},
			},
		}
	} else {
		// Count/filter query — return valid docs based on total
		response = map[string]any{
			"hits": map[string]any{
				"total": map[string]any{
					"value": float64(stats.docCount),
				},
			},
		}
	}

	data, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return fmt.Errorf("marshal mock response: %w", marshalErr)
	}

	if unmarshalErr := json.Unmarshal(data, result); unmarshalErr != nil {
		return fmt.Errorf("unmarshal mock response: %w", unmarshalErr)
	}

	return nil
}

func newTestWorstHandler(t *testing.T, es *mockESSearcher) *admin.BackfillWorstSourcesHandler {
	t.Helper()

	return admin.NewBackfillWorstSourcesHandler(
		nil, // sources client set per test
		es,
		nil, // job repo not needed for dry_run tests
		nil, // schedule computer not needed for dry_run tests
		infralogger.NewNop(),
		0, // use default stagger
	)
}

func setupWorstRequest(t *testing.T, queryParams string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/backfill/worst-sources?"+queryParams, http.NoBody)
	c.Request = req
	return c, w
}

// mockSourcesClient implements sources.Client for testing.
type mockSourcesClient struct {
	sources []*sources.SourceListItem
}

func (m *mockSourcesClient) ListSources(_ context.Context) ([]*sources.SourceListItem, error) {
	return m.sources, nil
}

func (m *mockSourcesClient) ListIndigenousSources(_ context.Context) ([]*sources.SourceListItem, error) {
	return []*sources.SourceListItem{}, nil
}

func (m *mockSourcesClient) GetSource(_ context.Context, _ uuid.UUID) (*sources.Source, error) {
	return &sources.Source{}, nil
}

func TestBackfillWorstSources_DryRun(t *testing.T) {
	es := newMockESSearcher()
	es.indexExists["good_source_raw_content"] = true
	es.indexStats["good_source_raw_content"] = mockIndexStats{avgWordCount: 500, docCount: 100}
	es.indexExists["bad_source_raw_content"] = true
	es.indexStats["bad_source_raw_content"] = mockIndexStats{avgWordCount: 20, docCount: 50}
	es.indexExists["medium_source_raw_content"] = true
	es.indexStats["medium_source_raw_content"] = mockIndexStats{avgWordCount: 150, docCount: 75}

	handler := newTestWorstHandler(t, es)
	handler.SourcesClient = &mockSourcesClient{
		sources: []*sources.SourceListItem{
			{ID: uuid.New(), Name: "Good Source", URL: "https://good.com", Enabled: true},
			{ID: uuid.New(), Name: "Bad Source", URL: "https://bad.com", Enabled: true},
			{ID: uuid.New(), Name: "Medium Source", URL: "https://medium.com", Enabled: true},
		},
	}

	c, w := setupWorstRequest(t, "dry_run=true&limit=20")
	handler.BackfillWorstSources(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report admin.WorstSourceReport
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &report); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}

	if report.SourcesFound != 3 {
		t.Errorf("expected 3 sources_found, got %d", report.SourcesFound)
	}
	if report.JobsDispatched != 0 {
		t.Errorf("expected 0 jobs_dispatched in dry_run, got %d", report.JobsDispatched)
	}
	if !report.DryRun {
		t.Error("expected dry_run=true")
	}

	// Verify sorted ascending by avg_word_count
	if len(report.Sources) < 2 {
		t.Fatal("expected at least 2 sources in report")
	}
	if report.Sources[0].AvgWordCount > report.Sources[1].AvgWordCount {
		t.Errorf("sources not sorted ascending: first=%f, second=%f",
			report.Sources[0].AvgWordCount, report.Sources[1].AvgWordCount)
	}
}

func TestBackfillWorstSources_Limit(t *testing.T) {
	es := newMockESSearcher()
	es.indexExists["s1_raw_content"] = true
	es.indexStats["s1_raw_content"] = mockIndexStats{avgWordCount: 10, docCount: 5}
	es.indexExists["s2_raw_content"] = true
	es.indexStats["s2_raw_content"] = mockIndexStats{avgWordCount: 20, docCount: 10}
	es.indexExists["s3_raw_content"] = true
	es.indexStats["s3_raw_content"] = mockIndexStats{avgWordCount: 30, docCount: 15}

	handler := newTestWorstHandler(t, es)
	handler.SourcesClient = &mockSourcesClient{
		sources: []*sources.SourceListItem{
			{ID: uuid.New(), Name: "S1", URL: "https://s1.com", Enabled: true},
			{ID: uuid.New(), Name: "S2", URL: "https://s2.com", Enabled: true},
			{ID: uuid.New(), Name: "S3", URL: "https://s3.com", Enabled: true},
		},
	}

	c, w := setupWorstRequest(t, "dry_run=true&limit=2")
	handler.BackfillWorstSources(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report admin.WorstSourceReport
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &report); decodeErr != nil {
		t.Fatalf("failed to decode: %v", decodeErr)
	}

	if report.SourcesFound != 2 {
		t.Errorf("expected 2 sources with limit=2, got %d", report.SourcesFound)
	}
}

func TestBackfillWorstSources_SkipsDisabled(t *testing.T) {
	es := newMockESSearcher()
	es.indexExists["enabled_raw_content"] = true
	es.indexStats["enabled_raw_content"] = mockIndexStats{avgWordCount: 50, docCount: 10}

	handler := newTestWorstHandler(t, es)
	handler.SourcesClient = &mockSourcesClient{
		sources: []*sources.SourceListItem{
			{ID: uuid.New(), Name: "Enabled", URL: "https://enabled.com", Enabled: true},
			{ID: uuid.New(), Name: "Disabled", URL: "https://disabled.com", Enabled: false},
		},
	}

	c, w := setupWorstRequest(t, "dry_run=true")
	handler.BackfillWorstSources(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report admin.WorstSourceReport
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &report); decodeErr != nil {
		t.Fatalf("failed to decode: %v", decodeErr)
	}

	if report.SourcesFound != 1 {
		t.Errorf("expected 1 source (disabled skipped), got %d", report.SourcesFound)
	}
}

func TestFilterEnabled(t *testing.T) {
	t.Helper()
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "A", Enabled: true},
		{ID: uuid.New(), Name: "B", Enabled: false},
		{ID: uuid.New(), Name: "C", Enabled: true},
	}

	result := admin.FilterEnabled(allSources)
	if len(result) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(result))
	}
}

func TestParseIntParam(t *testing.T) {
	t.Helper()
	tests := []struct {
		name       string
		input      string
		defaultVal int
		want       int
	}{
		{"empty", "", 20, 20},
		{"valid", "10", 20, 10},
		{"negative", "-5", 20, 20},
		{"invalid", "abc", 20, 20},
		{"zero", "0", 20, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			got := admin.ParseIntParam(tc.input, tc.defaultVal)
			if got != tc.want {
				t.Errorf("parseIntParam(%q, %d) = %d, want %d", tc.input, tc.defaultVal, got, tc.want)
			}
		})
	}
}

func TestGetValidationReport(t *testing.T) {
	es := newMockESSearcher()
	es.indexExists["source_a_raw_content"] = true
	es.indexStats["source_a_raw_content"] = mockIndexStats{avgWordCount: 200, docCount: 100}
	es.indexExists["source_b_raw_content"] = true
	es.indexStats["source_b_raw_content"] = mockIndexStats{avgWordCount: 10, docCount: 50}

	handler := newTestWorstHandler(t, es)
	handler.SourcesClient = &mockSourcesClient{
		sources: []*sources.SourceListItem{
			{ID: uuid.New(), Name: "Source A", URL: "https://a.com", Enabled: true},
			{ID: uuid.New(), Name: "Source B", URL: "https://b.com", Enabled: true},
		},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backfill/validation-report?min_word_count=100", http.NoBody)
	c.Request = req

	handler.GetValidationReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report admin.ValidationReport
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &report); decodeErr != nil {
		t.Fatalf("failed to decode: %v", decodeErr)
	}

	if len(report.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(report.Sources))
	}
}

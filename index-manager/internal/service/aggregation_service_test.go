//nolint:testpackage // Testing unexported methods requires same package access
package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// --- mock ES client ---

type mockESClient struct {
	searchResp *esapi.Response
	searchErr  error
	docCounts  []elasticsearch.IndexDocCount
	docErr     error
}

func (m *mockESClient) SearchAllClassifiedContent(_ context.Context, _ map[string]any) (*esapi.Response, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResp, nil
}

func (m *mockESClient) GetAllIndexDocCounts(_ context.Context) ([]elasticsearch.IndexDocCount, error) {
	if m.docErr != nil {
		return nil, m.docErr
	}
	return m.docCounts, nil
}

// --- mock logger (noop) ---

type noopLogger struct{}

func (n *noopLogger) Debug(_ string, _ ...infralogger.Field) {}
func (n *noopLogger) Info(_ string, _ ...infralogger.Field)  {}
func (n *noopLogger) Warn(_ string, _ ...infralogger.Field)  {}
func (n *noopLogger) Error(_ string, _ ...infralogger.Field) {}
func (n *noopLogger) Fatal(_ string, _ ...infralogger.Field) {}
func (n *noopLogger) With(_ ...infralogger.Field) infralogger.Logger {
	return n
}
func (n *noopLogger) Sync() error { return nil }

// --- helpers ---

func esapiResponse(t *testing.T, statusCode int, body string) *esapi.Response {
	t.Helper()
	return &esapi.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func newTestService(mock *mockESClient) *AggregationService {
	return NewAggregationService(mock, &noopLogger{})
}

// --- fetchClassifiedAggregations tests ---

func TestFetchClassifiedAggregations_ValidResponse(t *testing.T) {
	body := `{
		"aggregations": {
			"by_source": {
				"buckets": [
					{
						"key": "example_com",
						"avg_quality": {"value": 72.5},
						"recent_24h": {"doc_count": 15}
					},
					{
						"key": "news_org",
						"avg_quality": {"value": 55.0},
						"recent_24h": {"doc_count": 3}
					}
				]
			}
		}
	}`
	mock := &mockESClient{
		searchResp: esapiResponse(t, http.StatusOK, body),
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if len(qualityMap) != 2 {
		t.Fatalf("expected 2 quality entries, got %d", len(qualityMap))
	}
	if qualityMap["example_com"] != 72.5 {
		t.Errorf("expected example_com quality 72.5, got %f", qualityMap["example_com"])
	}
	if qualityMap["news_org"] != 55.0 {
		t.Errorf("expected news_org quality 55.0, got %f", qualityMap["news_org"])
	}
	if deltaMap["example_com"] != 15 {
		t.Errorf("expected example_com delta 15, got %d", deltaMap["example_com"])
	}
	if deltaMap["news_org"] != 3 {
		t.Errorf("expected news_org delta 3, got %d", deltaMap["news_org"])
	}
}

func TestFetchClassifiedAggregations_NullQuality(t *testing.T) {
	body := `{
		"aggregations": {
			"by_source": {
				"buckets": [
					{
						"key": "empty_source",
						"avg_quality": {"value": null},
						"recent_24h": {"doc_count": 0}
					}
				]
			}
		}
	}`
	mock := &mockESClient{
		searchResp: esapiResponse(t, http.StatusOK, body),
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if _, exists := qualityMap["empty_source"]; exists {
		t.Error("expected no quality entry for null value")
	}
	if deltaMap["empty_source"] != 0 {
		t.Errorf("expected delta 0, got %d", deltaMap["empty_source"])
	}
}

func TestFetchClassifiedAggregations_ESError(t *testing.T) {
	mock := &mockESClient{
		searchErr: io.ErrUnexpectedEOF,
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if len(qualityMap) != 0 {
		t.Errorf("expected empty qualityMap on ES error, got %d entries", len(qualityMap))
	}
	if len(deltaMap) != 0 {
		t.Errorf("expected empty deltaMap on ES error, got %d entries", len(deltaMap))
	}
}

func TestFetchClassifiedAggregations_ESErrorStatus(t *testing.T) {
	mock := &mockESClient{
		searchResp: esapiResponse(t, http.StatusBadRequest, `{"error":"fielddata disabled"}`),
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if len(qualityMap) != 0 {
		t.Errorf("expected empty qualityMap on 400 status, got %d entries", len(qualityMap))
	}
	if len(deltaMap) != 0 {
		t.Errorf("expected empty deltaMap on 400 status, got %d entries", len(deltaMap))
	}
}

func TestFetchClassifiedAggregations_MalformedJSON(t *testing.T) {
	mock := &mockESClient{
		searchResp: esapiResponse(t, http.StatusOK, `{not valid json`),
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if len(qualityMap) != 0 {
		t.Errorf("expected empty qualityMap on malformed JSON, got %d entries", len(qualityMap))
	}
	if len(deltaMap) != 0 {
		t.Errorf("expected empty deltaMap on malformed JSON, got %d entries", len(deltaMap))
	}
}

func TestFetchClassifiedAggregations_EmptyBuckets(t *testing.T) {
	body := `{"aggregations": {"by_source": {"buckets": []}}}`
	mock := &mockESClient{
		searchResp: esapiResponse(t, http.StatusOK, body),
	}
	svc := newTestService(mock)

	qualityMap, deltaMap := svc.fetchClassifiedAggregations(context.Background())

	if len(qualityMap) != 0 {
		t.Errorf("expected empty qualityMap, got %d entries", len(qualityMap))
	}
	if len(deltaMap) != 0 {
		t.Errorf("expected empty deltaMap, got %d entries", len(deltaMap))
	}
}

// --- GetSourceHealth tests ---

func TestGetSourceHealth_HappyPath(t *testing.T) {
	aggBody := `{
		"aggregations": {
			"by_source": {
				"buckets": [
					{
						"key": "example_com",
						"avg_quality": {"value": 72.5},
						"recent_24h": {"doc_count": 10}
					}
				]
			}
		}
	}`
	mock := &mockESClient{
		docCounts: []elasticsearch.IndexDocCount{
			{Name: "example_com_raw_content", DocCount: 100},
			{Name: "example_com_classified_content", DocCount: 80},
		},
		searchResp: esapiResponse(t, http.StatusOK, aggBody),
	}
	svc := newTestService(mock)

	resp, err := svc.GetSourceHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("expected 1 source, got %d", resp.Total)
	}

	src := resp.Sources[0]
	if src.Source != "example_com" {
		t.Errorf("expected source example_com, got %s", src.Source)
	}
	if src.RawCount != 100 {
		t.Errorf("expected raw count 100, got %d", src.RawCount)
	}
	if src.ClassifiedCount != 80 {
		t.Errorf("expected classified count 80, got %d", src.ClassifiedCount)
	}
	if src.Backlog != 20 {
		t.Errorf("expected backlog 20, got %d", src.Backlog)
	}
	if src.Delta24h != 10 {
		t.Errorf("expected delta24h 10, got %d", src.Delta24h)
	}
	if src.AvgQuality != 72.5 {
		t.Errorf("expected avg quality 72.5, got %f", src.AvgQuality)
	}
}

func TestGetSourceHealth_AggregationFailure_StillReturnsDocCounts(t *testing.T) {
	mock := &mockESClient{
		docCounts: []elasticsearch.IndexDocCount{
			{Name: "example_com_raw_content", DocCount: 50},
			{Name: "example_com_classified_content", DocCount: 30},
		},
		searchErr: io.ErrUnexpectedEOF,
	}
	svc := newTestService(mock)

	resp, err := svc.GetSourceHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("expected 1 source, got %d", resp.Total)
	}

	src := resp.Sources[0]
	if src.RawCount != 50 {
		t.Errorf("expected raw count 50, got %d", src.RawCount)
	}
	if src.ClassifiedCount != 30 {
		t.Errorf("expected classified count 30, got %d", src.ClassifiedCount)
	}
	if src.Delta24h != 0 {
		t.Errorf("expected delta24h 0 on aggregation failure, got %d", src.Delta24h)
	}
	if src.AvgQuality != 0 {
		t.Errorf("expected avg quality 0 on aggregation failure, got %f", src.AvgQuality)
	}
}

func TestGetSourceHealth_DocCountsFailure(t *testing.T) {
	mock := &mockESClient{
		docErr: io.ErrUnexpectedEOF,
	}
	svc := newTestService(mock)

	_, err := svc.GetSourceHealth(context.Background())
	if err == nil {
		t.Fatal("expected error when doc counts fail")
	}
}

// --- buildSourceCountMaps tests ---

func TestBuildSourceCountMaps(t *testing.T) {
	svc := newTestService(&mockESClient{})

	docCounts := []elasticsearch.IndexDocCount{
		{Name: "site_a_raw_content", DocCount: 100},
		{Name: "site_a_classified_content", DocCount: 80},
		{Name: "site_b_raw_content", DocCount: 50},
		{Name: "unrelated_index", DocCount: 999},
	}

	raw, classified := svc.buildSourceCountMaps(docCounts)

	if raw["site_a"] != 100 {
		t.Errorf("expected site_a raw 100, got %d", raw["site_a"])
	}
	if classified["site_a"] != 80 {
		t.Errorf("expected site_a classified 80, got %d", classified["site_a"])
	}
	if raw["site_b"] != 50 {
		t.Errorf("expected site_b raw 50, got %d", raw["site_b"])
	}
	if _, exists := classified["site_b"]; exists {
		t.Error("site_b should not have classified entry")
	}
	if _, exists := raw["unrelated_index"]; exists {
		t.Error("unrelated_index should not appear in raw map")
	}
}

// --- mergeSourceNames tests ---

func TestMergeSourceNames(t *testing.T) {
	svc := newTestService(&mockESClient{})

	raw := map[string]int64{"a": 1, "b": 2}
	classified := map[string]int64{"b": 3, "c": 4}

	sources := svc.mergeSourceNames(raw, classified)

	if len(sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(sources))
	}

	seen := make(map[string]bool)
	for _, s := range sources {
		seen[s] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !seen[expected] {
			t.Errorf("expected source %q in merged list", expected)
		}
	}
}

// --- assembleSourceHealthList tests ---

func TestAssembleSourceHealthList_BacklogClamped(t *testing.T) {
	svc := newTestService(&mockESClient{})

	result := svc.assembleSourceHealthList(
		[]string{"src"},
		map[string]int64{"src": 10},
		map[string]int64{"src": 50},
		map[string]float64{"src": 60.0},
		map[string]int64{"src": 5},
	)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Backlog != 0 {
		t.Errorf("expected backlog clamped to 0 when classified > raw, got %d", result[0].Backlog)
	}
	if result[0].Delta24h != 5 {
		t.Errorf("expected delta24h 5, got %d", result[0].Delta24h)
	}
	if result[0].AvgQuality != 60.0 {
		t.Errorf("expected avg quality 60.0, got %f", result[0].AvgQuality)
	}
}

func TestAssembleSourceHealthList_MissingAggregations(t *testing.T) {
	svc := newTestService(&mockESClient{})

	result := svc.assembleSourceHealthList(
		[]string{"src"},
		map[string]int64{"src": 100},
		map[string]int64{"src": 90},
		map[string]float64{},
		map[string]int64{},
	)

	if result[0].AvgQuality != 0 {
		t.Errorf("expected zero avg quality when absent from map, got %f", result[0].AvgQuality)
	}
	if result[0].Delta24h != 0 {
		t.Errorf("expected zero delta24h when absent from map, got %d", result[0].Delta24h)
	}
}

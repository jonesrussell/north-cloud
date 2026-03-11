package insights_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestParseSummaryBuckets(t *testing.T) {
	body := `{
		"aggregations": {
			"recent_summaries": {
				"buckets": [
					{"key": "Domain X borderline rate 40%", "doc_count": 5},
					{"key": "Label Y low confidence", "doc_count": 3}
				]
			}
		}
	}`

	result, err := insights.ParseSummaryBucketsForTest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parseSummaryBuckets() error = %v", err)
	}

	const expectedBuckets = 2
	if len(result) != expectedBuckets {
		t.Fatalf("expected %d summaries, got %d", expectedBuckets, len(result))
	}

	if !result["Domain X borderline rate 40%"] {
		t.Error("expected 'Domain X borderline rate 40%' in result")
	}
	if !result["Label Y low confidence"] {
		t.Error("expected 'Label Y low confidence' in result")
	}
}

func TestParseSummaryBuckets_Empty(t *testing.T) {
	body := `{"aggregations": {"recent_summaries": {"buckets": []}}}`

	result, err := insights.ParseSummaryBucketsForTest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestDeduplicator_Filter_NoCooldown(t *testing.T) {
	// With cooldownHours=0, filter should pass everything through.
	d := insights.NewDeduplicator(nil, 0)

	insightList := []category.Insight{
		{Summary: "test insight"},
	}

	result, err := d.Filter(t.Context(), insightList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 insight, got %d", len(result))
	}
}

func TestDeduplicator_Filter_EmptyInput(t *testing.T) {
	d := insights.NewDeduplicator(nil, 24)

	result, err := d.Filter(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 insights, got %d", len(result))
	}
}

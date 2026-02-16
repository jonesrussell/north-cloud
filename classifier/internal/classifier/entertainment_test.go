// classifier/internal/classifier/entertainment_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/entertainmentmlclient"
)

type mockEntertainmentMLClient struct {
	response *entertainmentmlclient.ClassifyResponse
	err      error
}

func (m *mockEntertainmentMLClient) Classify(_ context.Context, _, _ string) (*entertainmentmlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockEntertainmentMLClient) Health(_ context.Context) error {
	return nil
}

func TestEntertainmentClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	ec := NewEntertainmentClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "New film premieres at Cannes",
	}

	result, err := ec.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestEntertainmentClassifier_Classify_RulesOnly_Core(t *testing.T) {
	t.Helper()

	ec := NewEntertainmentClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Oscar-winning film premieres at Cannes",
		RawText: "The movie earned rave reviews from critics.",
	}

	result, err := ec.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when rules match")
	}
	if result.Relevance != entertainmentRelevanceCore {
		t.Errorf("expected core_entertainment, got %s", result.Relevance)
	}
}

// Test constants for entertainment decision context tests.
const (
	testEntertainmentMLProcessingTimeMs = 22
	testEntertainmentMLConfidence       = 0.80
	testEntertainmentExpectedCategories = 2
)

func TestEntertainmentClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockEntertainmentMLClient{
		response: &entertainmentmlclient.ClassifyResponse{
			Relevance:           "core_entertainment",
			RelevanceConfidence: testEntertainmentMLConfidence,
			Categories:          []string{"film", "awards"},
			ModelVersion:        "2026-02-10-entertainment-v1",
			ProcessingTimeMs:    testEntertainmentMLProcessingTimeMs,
		},
	}

	ec := NewEntertainmentClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-dc-both",
		Title:   "Oscar-winning film premieres at Cannes",
		RawText: "The movie earned rave reviews from critics.",
	}

	result, err := ec.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	verifyEntertainmentDecisionPath(t, result, "both_agree")
	verifyEntertainmentMLConfidencePopulated(t, result)
	verifyEntertainmentProcessingTimeMs(t, result, testEntertainmentMLProcessingTimeMs)

	if len(result.Categories) != testEntertainmentExpectedCategories {
		t.Errorf("expected %d categories, got %d",
			testEntertainmentExpectedCategories, len(result.Categories))
	}
}

func TestEntertainmentClassifier_Classify_RulesOnly_NotRelevant(t *testing.T) {
	t.Helper()

	ec := NewEntertainmentClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Weather forecast for the weekend",
		RawText: "Sunny skies expected.",
	}

	result, err := ec.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != entertainmentRelevanceNot {
		t.Errorf("expected not_entertainment, got %s", result.Relevance)
	}
}

func verifyEntertainmentDecisionPath(t *testing.T, result *domain.EntertainmentResult, expected string) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

func verifyEntertainmentMLConfidencePopulated(t *testing.T, result *domain.EntertainmentResult) {
	t.Helper()

	if result.MLConfidenceRaw == 0 {
		t.Error("expected MLConfidenceRaw to be populated when ML is available")
	}
}

func verifyEntertainmentProcessingTimeMs(t *testing.T, result *domain.EntertainmentResult, expected int64) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

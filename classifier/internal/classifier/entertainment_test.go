// classifier/internal/classifier/entertainment_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

type mockEntertainmentMLClient struct {
	response *mlclient.StandardResponse
	err      error
}

func (m *mockEntertainmentMLClient) Classify(
	_ context.Context, _, _ string,
) (*mlclient.StandardResponse, error) {
	return m.response, m.err
}

// newEntertainmentMLResponse creates a StandardResponse with entertainment-specific fields.
func newEntertainmentMLResponse(
	relevance, modelVersion string, confidence float64,
	categories []string, processingMs float64,
) *mlclient.StandardResponse {
	result, marshalErr := json.Marshal(entertainmentMLResponse{
		Categories: categories,
	})
	if marshalErr != nil {
		panic("test marshal failed: " + marshalErr.Error())
	}
	return &mlclient.StandardResponse{
		Module:           "entertainment",
		Version:          modelVersion,
		SchemaVersion:    "1.0",
		Result:           result,
		Relevance:        float64Ptr(mapEntertainmentRelevanceToScore(relevance)),
		Confidence:       float64Ptr(confidence),
		ProcessingTimeMs: processingMs,
		RequestID:        "test",
	}
}

// mapEntertainmentRelevanceToScore maps an entertainment relevance class to a numeric score.
func mapEntertainmentRelevanceToScore(relevance string) float64 {
	switch relevance {
	case "core_entertainment":
		return 0.9
	case "peripheral_entertainment":
		return 0.5
	default:
		return 0.1
	}
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
		response: newEntertainmentMLResponse(
			"core_entertainment", "2026-02-10-entertainment-v1",
			testEntertainmentMLConfidence,
			[]string{"film", "awards"}, testEntertainmentMLProcessingTimeMs,
		),
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

func verifyEntertainmentDecisionPath(
	t *testing.T, result *domain.EntertainmentResult, expected string,
) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

func verifyEntertainmentMLConfidencePopulated(
	t *testing.T, result *domain.EntertainmentResult,
) {
	t.Helper()

	if result.MLConfidenceRaw == 0 {
		t.Error("expected MLConfidenceRaw to be populated when ML is available")
	}
}

func verifyEntertainmentProcessingTimeMs(
	t *testing.T, result *domain.EntertainmentResult, expected int64,
) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

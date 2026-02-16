//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

type mockCoforgeMLClient struct {
	response *coforgemlclient.ClassifyResponse
	err      error
}

func (m *mockCoforgeMLClient) Classify(_ context.Context, _, _ string) (*coforgemlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockCoforgeMLClient) Health(_ context.Context) error {
	return nil
}

func TestCoforgeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "AI startup releases developer SDK",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestCoforgeClassifier_Classify_RulesOnly_Peripheral(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Series A funding round announced",
		RawText: "The startup raised $5M.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when rules match")
	}
	if result.Relevance != coforgeRelevancePeripheral {
		t.Errorf("expected peripheral, got %s", result.Relevance)
	}
}

func TestCoforgeClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockCoforgeMLClient{
		response: &coforgemlclient.ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: 0.92,
			Audience:            "hybrid",
			AudienceConfidence:  0.78,
			Topics:              []string{"funding_round", "devtools"},
			Industries:          []string{"saas"},
			ModelVersion:        "2026-02-08-coforge-v1",
		},
	}

	cc := NewCoforgeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Startup open-sources developer SDK after Series A",
		RawText: "The company raised $10M and released their API toolkit.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != coforgeRelevanceCore {
		t.Errorf("expected core_coforge, got %s", result.Relevance)
	}
	if result.Audience != "hybrid" {
		t.Errorf("expected hybrid, got %s", result.Audience)
	}
	if len(result.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(result.Topics))
	}
}

// Test constants for coforge decision context tests.
const (
	testCoforgeMLProcessingTimeMs = 28
	testCoforgeMLConfidence       = 0.82
	testCoforgeExpectedTopics     = 2
)

func TestCoforgeClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockCoforgeMLClient{
		response: &coforgemlclient.ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: testCoforgeMLConfidence,
			Audience:            "hybrid",
			AudienceConfidence:  0.78,
			Topics:              []string{"funding_round", "devtools"},
			Industries:          []string{"saas"},
			ModelVersion:        "2026-02-08-coforge-v1",
			ProcessingTimeMs:    testCoforgeMLProcessingTimeMs,
		},
	}

	cc := NewCoforgeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-dc-both",
		Title:   "Startup open-sources developer SDK after Series A",
		RawText: "The company raised $10M and released their API toolkit.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	verifyCoforgeDecisionPath(t, result, "both_agree")
	verifyCoforgeMLConfidencePopulated(t, result)
	verifyCoforgeProcessingTimeMs(t, result, testCoforgeMLProcessingTimeMs)

	if len(result.Topics) != testCoforgeExpectedTopics {
		t.Errorf("expected %d topics, got %d",
			testCoforgeExpectedTopics, len(result.Topics))
	}
}

func verifyCoforgeDecisionPath(t *testing.T, result *domain.CoforgeResult, expected string) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

func verifyCoforgeMLConfidencePopulated(t *testing.T, result *domain.CoforgeResult) {
	t.Helper()

	if result.MLConfidenceRaw == 0 {
		t.Error("expected MLConfidenceRaw to be populated when ML is available")
	}
}

func verifyCoforgeProcessingTimeMs(t *testing.T, result *domain.CoforgeResult, expected int64) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

func TestCoforgeClassifier_Classify_RulesOnly_NotRelevant(t *testing.T) {
	t.Helper()

	cc := NewCoforgeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-4",
		Title:   "Weather forecast for the weekend",
		RawText: "Sunny skies expected.",
	}

	result, err := cc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != coforgeRelevanceNot {
		t.Errorf("expected not_relevant, got %s", result.Relevance)
	}
}

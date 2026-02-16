// classifier/internal/classifier/anishinaabe_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/anishinaabemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

type mockAnishinaabeMLClient struct {
	response *anishinaabemlclient.ClassifyResponse
	err      error
}

func (m *mockAnishinaabeMLClient) Classify(_ context.Context, _, _ string) (*anishinaabemlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockAnishinaabeMLClient) Health(_ context.Context) error {
	return nil
}

func TestAnishinaabeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	ac := NewAnishinaabeClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "Anishinaabe community celebrates language revitalization",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestAnishinaabeClassifier_Classify_RulesOnly_Core(t *testing.T) {
	t.Helper()

	ac := NewAnishinaabeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Anishinaabe community celebrates language revitalization",
		RawText: "Elders lead Anishinaabemowin workshops in the community.",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when rules match")
	}
	if result.Relevance != anishinaabeRelevanceCore {
		t.Errorf("expected core_anishinaabe, got %s", result.Relevance)
	}
}

// Test constants for anishinaabe decision context tests.
const (
	testAnishinaabeMLProcessingTimeMs = 18
	testAnishinaabeMLConfidence       = 0.85
	testAnishinaabeExpectedCategories = 2
)

func TestAnishinaabeClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockAnishinaabeMLClient{
		response: &anishinaabemlclient.ClassifyResponse{
			Relevance:           "core_anishinaabe",
			RelevanceConfidence: testAnishinaabeMLConfidence,
			Categories:          []string{"culture", "language"},
			ModelVersion:        "2026-02-16-anishinaabe-v1",
			ProcessingTimeMs:    testAnishinaabeMLProcessingTimeMs,
		},
	}

	ac := NewAnishinaabeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-dc-both",
		Title:   "Anishinaabe community celebrates language revitalization",
		RawText: "Elders lead Anishinaabemowin workshops in the community.",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	verifyAnishinaabeDecisionPath(t, result, "both_agree")
	verifyAnishinaabeMLConfidencePopulated(t, result)
	verifyAnishinaabeProcessingTimeMs(t, result, testAnishinaabeMLProcessingTimeMs)

	if len(result.Categories) != testAnishinaabeExpectedCategories {
		t.Errorf("expected %d categories, got %d",
			testAnishinaabeExpectedCategories, len(result.Categories))
	}
}

func TestAnishinaabeClassifier_Classify_RulesOnly_NotRelevant(t *testing.T) {
	t.Helper()

	ac := NewAnishinaabeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Weather forecast for the weekend",
		RawText: "Sunny skies expected.",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != anishinaabeRelevanceNot {
		t.Errorf("expected not_anishinaabe, got %s", result.Relevance)
	}
}

func verifyAnishinaabeDecisionPath(t *testing.T, result *domain.AnishinaabeResult, expected string) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

func verifyAnishinaabeMLConfidencePopulated(t *testing.T, result *domain.AnishinaabeResult) {
	t.Helper()

	if result.MLConfidenceRaw == 0 {
		t.Error("expected MLConfidenceRaw to be populated when ML is available")
	}
}

func verifyAnishinaabeProcessingTimeMs(t *testing.T, result *domain.AnishinaabeResult, expected int64) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

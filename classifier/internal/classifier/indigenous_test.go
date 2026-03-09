// classifier/internal/classifier/indigenous_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/indigenousmlclient"
)

type mockIndigenousMLClient struct {
	response *indigenousmlclient.ClassifyResponse
	err      error
}

func (m *mockIndigenousMLClient) Classify(_ context.Context, _, _ string) (*indigenousmlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockIndigenousMLClient) Health(_ context.Context) error {
	return nil
}

func TestIndigenousClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, false)

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

func TestIndigenousClassifier_Classify_RulesOnly_Core(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, true)

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
	if result.Relevance != indigenousRelevanceCore {
		t.Errorf("expected core_indigenous, got %s", result.Relevance)
	}
}

// Test constants for indigenous decision context tests.
const (
	testIndigenousMLProcessingTimeMs = 18
	testIndigenousMLConfidence       = 0.85
	testIndigenousExpectedCategories = 2
)

func TestIndigenousClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockIndigenousMLClient{
		response: &indigenousmlclient.ClassifyResponse{
			Relevance:           "core_indigenous",
			RelevanceConfidence: testIndigenousMLConfidence,
			Categories:          []string{"culture", "language"},
			ModelVersion:        "2026-02-27-indigenous-v1",
			ProcessingTimeMs:    testIndigenousMLProcessingTimeMs,
		},
	}

	ac := NewIndigenousClassifier(mlMock, &mockLogger{}, true)

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

	verifyIndigenousDecisionPath(t, result, "both_agree")
	verifyIndigenousMLConfidencePopulated(t, result)
	verifyIndigenousProcessingTimeMs(t, result, testIndigenousMLProcessingTimeMs)

	if len(result.Categories) != testIndigenousExpectedCategories {
		t.Errorf("expected %d categories, got %d",
			testIndigenousExpectedCategories, len(result.Categories))
	}
}

func TestIndigenousClassifier_RegionPassthrough(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-region",
		Title:   "First Nations community event",
		RawText: "Indigenous peoples gather for ceremony.",
		Meta: map[string]any{
			"indigenous_region": "canada",
		},
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Region != "canada" {
		t.Errorf("expected region=canada, got %q", result.Region)
	}
}

func TestIndigenousClassifier_RegionPassthrough_Empty(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-region-empty",
		Title:   "First Nations community event",
		RawText: "Indigenous peoples gather for ceremony.",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Region != "" {
		t.Errorf("expected empty region when no meta, got %q", result.Region)
	}
}

func TestIndigenousClassifier_Classify_RulesOnly_NotRelevant(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, true)

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
	if result.Relevance != indigenousRelevanceNot {
		t.Errorf("expected not_indigenous, got %s", result.Relevance)
	}
}

func TestIndigenousClassifier_ConfidencePassthrough(t *testing.T) {
	t.Helper()

	ac := NewIndigenousClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-confidence",
		Title:   "Anishinaabe community celebrates culture and language",
		RawText: "Elders lead Anishinaabemowin workshops in the community.",
	}

	result, err := ac.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.FinalConfidence < indigenousConfidenceCoreBase {
		t.Errorf("expected FinalConfidence >= %f, got %f",
			indigenousConfidenceCoreBase, result.FinalConfidence)
	}
	if result.FinalConfidence > indigenousConfidenceCoreMax {
		t.Errorf("expected FinalConfidence <= %f, got %f",
			indigenousConfidenceCoreMax, result.FinalConfidence)
	}
}

func verifyIndigenousDecisionPath(t *testing.T, result *domain.IndigenousResult, expected string) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

func verifyIndigenousMLConfidencePopulated(t *testing.T, result *domain.IndigenousResult) {
	t.Helper()

	if result.MLConfidenceRaw == 0 {
		t.Error("expected MLConfidenceRaw to be populated when ML is available")
	}
}

func verifyIndigenousProcessingTimeMs(t *testing.T, result *domain.IndigenousResult, expected int64) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

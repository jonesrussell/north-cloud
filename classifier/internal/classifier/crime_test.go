// classifier/internal/classifier/crime_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

type mockMLClient struct {
	response *mlclient.ClassifyResponse
	err      error
}

func (m *mockMLClient) Classify(_ context.Context, _, _ string) (*mlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockMLClient) Health(_ context.Context) error {
	return nil
}

func TestCrimeClassifier_Classify_RulesOnly(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-1",
		Title:   "Man charged with murder after stabbing",
		RawText: "Police arrested a suspect.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	if !result.HomepageEligible {
		t.Error("expected homepage eligible for high-confidence crime")
	}
}

func TestCrimeClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMLClient{
		response: &mlclient.ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			Location:            "local_canada",
		},
	}

	sc := NewCrimeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Man charged with murder",
		RawText: "Downtown stabbing incident.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	// Both agree, should have high confidence
	if result.FinalConfidence < 0.75 {
		t.Errorf("expected confidence >= 0.75 when both agree, got %f", result.FinalConfidence)
	}
}

func TestCrimeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-3",
		Title: "Murder headline",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestCrimeClassifier_SubLabel_CriminalJustice(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Criminal justice: international crime with court proceedings
	// Uses U.S. to trigger international downgrade to peripheral_crime
	raw := &domain.RawContent{
		ID:      "test-cj-1",
		Title:   "U.S. man sentenced to 10 years for stabbing",
		RawText: "The court ordered the defendant to serve time. The prosecutor praised the sentence.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != relevancePeripheral {
		t.Errorf("expected peripheral_crime, got %s", result.Relevance)
	}
	if result.SubLabel != SubLabelCriminalJustice {
		t.Errorf("expected criminal_justice sub_label, got %s", result.SubLabel)
	}
}

func TestCrimeClassifier_SubLabel_CrimeContext(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Crime context: international crime with document release
	// Uses Minneapolis to trigger international downgrade to peripheral_crime
	raw := &domain.RawContent{
		ID:      "test-cc-1",
		Title:   "Minneapolis police release shooting video from fatal incident",
		RawText: "The declassified footage reveals details from the historical case.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != relevancePeripheral {
		t.Errorf("expected peripheral_crime, got %s", result.Relevance)
	}
	if result.SubLabel != SubLabelCrimeContext {
		t.Errorf("expected crime_context sub_label, got %s", result.SubLabel)
	}
}

func TestCrimeClassifier_SubLabel_CoreStreetCrime_NoSubLabel(t *testing.T) {
	t.Helper()

	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	// Core street crime should have no sub_label
	raw := &domain.RawContent{
		ID:      "test-core-1",
		Title:   "Man charged with murder after shooting",
		RawText: "Police arrested a suspect at the scene.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Relevance != relevanceCoreStreetCrime {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}
	if result.SubLabel != "" {
		t.Errorf("expected empty sub_label for core_street_crime, got %s", result.SubLabel)
	}
}

// ML processing time used in decision context tests.
const testMLProcessingTimeMs = 42

func TestCrimeClassifier_DecisionContext_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMLClient{
		response: &mlclient.ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.88,
			CrimeTypes:          []string{"violent_crime"},
			Location:            "local_canada",
			ProcessingTimeMs:    testMLProcessingTimeMs,
		},
	}

	sc := NewCrimeClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-dc-both",
		Title:   "Man charged with murder after stabbing",
		RawText: "Police arrested a suspect downtown.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertDecisionPath(t, result, "both_agree")
	assertMLConfidencePopulated(t, result)
	assertProcessingTimeMs(t, result, testMLProcessingTimeMs)
}

func TestCrimeClassifier_DecisionContext_RulesOnly(t *testing.T) {
	t.Helper()

	// No ML client -> rules_only path
	sc := NewCrimeClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-dc-rules",
		Title:   "Man charged with murder after shooting",
		RawText: "Police arrested a suspect at the scene.",
	}

	result, err := sc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertDecisionPath(t, result, "rules_only")

	// No ML -> MLConfidence should be zero
	if result.MLConfidence != 0 {
		t.Errorf("expected MLConfidence=0 without ML, got %f", result.MLConfidence)
	}

	// No ML -> ProcessingTimeMs should be zero
	if result.ProcessingTimeMs != 0 {
		t.Errorf("expected ProcessingTimeMs=0 without ML, got %d", result.ProcessingTimeMs)
	}
}

// assertDecisionPath verifies the decision path on a CrimeResult.
func assertDecisionPath(t *testing.T, result *CrimeResult, expected string) {
	t.Helper()

	if result.DecisionPath != expected {
		t.Errorf("expected DecisionPath=%q, got %q", expected, result.DecisionPath)
	}
}

// assertMLConfidencePopulated verifies MLConfidence is non-zero.
func assertMLConfidencePopulated(t *testing.T, result *CrimeResult) {
	t.Helper()

	if result.MLConfidence == 0 {
		t.Error("expected MLConfidence to be populated when ML is available")
	}
}

// assertProcessingTimeMs verifies the processing time matches the expected value.
func assertProcessingTimeMs(t *testing.T, result *CrimeResult, expected int64) {
	t.Helper()

	if result.ProcessingTimeMs != expected {
		t.Errorf("expected ProcessingTimeMs=%d, got %d", expected, result.ProcessingTimeMs)
	}
}

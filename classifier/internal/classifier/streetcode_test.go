// classifier/internal/classifier/streetcode_test.go
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

func TestStreetCodeClassifier_Classify_RulesOnly(t *testing.T) {
	t.Helper()

	sc := NewStreetCodeClassifier(nil, &mockLogger{}, true)

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

func TestStreetCodeClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMLClient{
		response: &mlclient.ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			Location:            "local_canada",
		},
	}

	sc := NewStreetCodeClassifier(mlMock, &mockLogger{}, true)

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

func TestStreetCodeClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	sc := NewStreetCodeClassifier(nil, &mockLogger{}, false)

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

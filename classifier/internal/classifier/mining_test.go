// classifier/internal/classifier/mining_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/miningmlclient"
)

type mockMiningMLClient struct {
	response *miningmlclient.ClassifyResponse
	err      error
}

func (m *mockMiningMLClient) Classify(_ context.Context, _, _ string) (*miningmlclient.ClassifyResponse, error) {
	return m.response, m.err
}

func (m *mockMiningMLClient) Health(_ context.Context) error {
	return nil
}

func TestMiningClassifier_Classify_Disabled(t *testing.T) {
	t.Helper()

	mc := NewMiningClassifier(nil, &mockLogger{}, false)

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "Gold exploration in Ontario",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestMiningClassifier_Classify_RulesOnly_Core(t *testing.T) {
	t.Helper()

	mc := NewMiningClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Gold exploration drill results in Ontario",
		RawText: "Assay results show high-grade intercept.",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when rules match")
	}
	if result.Relevance != miningRelevanceCore {
		t.Errorf("expected core_mining, got %s", result.Relevance)
	}
}

func TestMiningClassifier_Classify_BothAgree(t *testing.T) {
	t.Helper()

	mlMock := &mockMiningMLClient{
		response: &miningmlclient.ClassifyResponse{
			Relevance:           "core_mining",
			RelevanceConfidence: 0.93,
			MiningStage:         "exploration",
			Commodities:         []string{"gold", "copper"},
			Location:            "local_canada",
			ModelVersion:        "2025-02-01-mining-v1",
		},
	}

	mc := NewMiningClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Gold exploration drill results assay",
		RawText: "Ontario mining project.",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != miningRelevanceCore {
		t.Errorf("expected core_mining, got %s", result.Relevance)
	}
	if result.MiningStage != "exploration" {
		t.Errorf("expected exploration, got %s", result.MiningStage)
	}
	if len(result.Commodities) != 2 {
		t.Errorf("expected 2 commodities, got %d", len(result.Commodities))
	}
	if result.ModelVersion != "2025-02-01-mining-v1" {
		t.Errorf("expected model_version, got %s", result.ModelVersion)
	}
}

func TestMiningClassifier_Classify_RuleCore_MLNotMining(t *testing.T) {
	t.Helper()

	mlMock := &mockMiningMLClient{
		response: &miningmlclient.ClassifyResponse{
			Relevance:           "not_mining",
			RelevanceConfidence: 0.8,
		},
	}

	mc := NewMiningClassifier(mlMock, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-4",
		Title:   "Gold exploration drill results assay",
		RawText: "Ontario mining project.",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != miningRelevanceCore {
		t.Errorf("expected core_mining (rule wins), got %s", result.Relevance)
	}
	if !result.ReviewRequired {
		t.Error("expected ReviewRequired when rule and ML disagree")
	}
}

func TestMiningClassifier_Classify_RulesOnly_NotMining(t *testing.T) {
	t.Helper()

	mc := NewMiningClassifier(nil, &mockLogger{}, true)

	raw := &domain.RawContent{
		ID:      "test-5",
		Title:   "Weather forecast for the weekend",
		RawText: "Sunny skies expected.",
	}

	result, err := mc.Classify(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Relevance != miningRelevanceNot {
		t.Errorf("expected not_mining, got %s", result.Relevance)
	}
}

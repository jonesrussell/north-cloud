package drift_test

import (
	"encoding/json"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestBaseline_JSONRoundTrip(t *testing.T) {
	t.Helper()
	baseline := &drift.Baseline{
		ComputedAt:  "2026-03-08T06:00:00Z",
		WindowDays:  7,
		SampleCount: 100,
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.5, "politics": 0.3},
		},
		ConfidenceHistograms: map[string][]float64{
			"global": {0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		},
		CrossMatrix: map[string]map[string]float64{
			"north_america": {"tech": 0.5},
		},
		CrossMatrixCounts: map[string]map[string]int{
			"north_america": {"tech": 50},
		},
	}

	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got drift.Baseline
	unmarshalErr := json.Unmarshal(data, &got)
	if unmarshalErr != nil {
		t.Fatalf("unmarshal: %v", unmarshalErr)
	}

	if got.SampleCount != 100 {
		t.Errorf("expected SampleCount 100, got %d", got.SampleCount)
	}
	if got.CategoryDistribution["global"]["tech"] != 0.5 {
		t.Errorf("expected global/tech 0.5, got %f", got.CategoryDistribution["global"]["tech"])
	}
}

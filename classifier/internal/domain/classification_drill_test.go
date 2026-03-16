package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestDrillResult_JSONSerialization(t *testing.T) {
	dr := domain.DrillResult{
		HoleID:     "DDH-24-001",
		Commodity:  "gold",
		InterceptM: 12.5,
		Grade:      3.2,
		Unit:       "g/t",
	}
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got domain.DrillResult
	if unmarshalErr := json.Unmarshal(data, &got); unmarshalErr != nil {
		t.Fatalf("unmarshal: %v", unmarshalErr)
	}
	if got != dr {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, dr)
	}
}

func TestMiningResult_DrillResults_OmitEmpty(t *testing.T) {
	mr := domain.MiningResult{
		Relevance:       "core_mining",
		MiningStage:     "exploration",
		Commodities:     []string{"gold"},
		Location:        "national_canada",
		FinalConfidence: 0.95,
	}
	data, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "drill_results") {
		t.Error("expected drill_results to be omitted when empty")
	}
	if strings.Contains(s, "extraction_method") {
		t.Error("expected extraction_method to be omitted when empty")
	}
}

func TestMiningResult_DrillResults_Present(t *testing.T) {
	mr := domain.MiningResult{
		Relevance:   "core_mining",
		Commodities: []string{"gold"},
		DrillResults: []domain.DrillResult{
			{HoleID: "DDH-24-001", Commodity: "gold", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"},
		},
		ExtractionMethod: "regex",
	}
	data, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"drill_results"`) {
		t.Error("expected drill_results in JSON")
	}
	if !strings.Contains(s, `"extraction_method":"regex"`) {
		t.Error("expected extraction_method in JSON")
	}
}

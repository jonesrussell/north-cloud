package router

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMiningData_DrillResults_JSONRoundTrip(t *testing.T) {
	md := MiningData{
		Relevance:   "core_mining",
		Commodities: []string{"gold"},
		DrillResults: []DrillResult{
			{HoleID: "DDH-24-001", Commodity: "gold", InterceptM: 12.5, Grade: 3.2, Unit: "g/t"},
			{HoleID: "DDH-24-002", Commodity: "copper", InterceptM: 8.0, Grade: 1.5, Unit: "%"},
		},
		ExtractionMethod: "hybrid",
	}

	data, err := json.Marshal(md)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got MiningData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.DrillResults) != 2 {
		t.Fatalf("got %d drill results, want 2", len(got.DrillResults))
	}
	if got.DrillResults[0].HoleID != "DDH-24-001" {
		t.Errorf("first result HoleID = %q, want DDH-24-001", got.DrillResults[0].HoleID)
	}
	if got.ExtractionMethod != "hybrid" {
		t.Errorf("ExtractionMethod = %q, want hybrid", got.ExtractionMethod)
	}
}

func TestMiningData_NoDrillResults_OmitEmpty(t *testing.T) {
	md := MiningData{
		Relevance:   "core_mining",
		Commodities: []string{"gold"},
	}
	data, err := json.Marshal(md)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "drill_results") {
		t.Error("expected drill_results to be omitted when nil")
	}
}


package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestExtractDrillResults_InterceptAtGrade(t *testing.T) {
	body := `Drill hole DDH-24-001 returned 12.5m @ 3.2 g/t Au from 45.0m.`
	results, confidence := extractDrillRegex(body)

	if confidence != drillConfidenceComplete {
		t.Errorf("confidence = %q, want %q", confidence, drillConfidenceComplete)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.HoleID != "DDH-24-001" {
		t.Errorf("HoleID = %q, want DDH-24-001", r.HoleID)
	}
	if r.InterceptM != 12.5 {
		t.Errorf("InterceptM = %f, want 12.5", r.InterceptM)
	}
	if r.Grade != 3.2 {
		t.Errorf("Grade = %f, want 3.2", r.Grade)
	}
	if r.Unit != "g/t" {
		t.Errorf("Unit = %q, want g/t", r.Unit)
	}
}

func TestExtractDrillResults_MultipleHoles(t *testing.T) {
	body := `Highlights include:
	DDH-24-001: 12.5m @ 3.2 g/t Au
	DDH-24-002: 8.0m @ 1.5% Cu
	RC-001: 15.0 metres @ 0.8 g/t Au`
	results, confidence := extractDrillRegex(body)

	if confidence != drillConfidenceComplete {
		t.Errorf("confidence = %q, want %q", confidence, drillConfidenceComplete)
	}
	if len(results) < 3 {
		t.Errorf("got %d results, want >= 3", len(results))
	}
}

func TestExtractDrillResults_FromToInterval(t *testing.T) {
	body := `Hole BH-001 intersected gold mineralization from 45.0m to 57.5m grading 2.1 g/t Au`
	results, confidence := extractDrillRegex(body)

	if confidence == drillConfidenceNone {
		t.Error("expected some results from from-to pattern")
	}
	// The from-to pattern should produce a result with intercept 12.5 (57.5 - 45.0)
	found := false
	for _, r := range results {
		if r.InterceptM == 12.5 {
			found = true
			break
		}
	}
	if len(results) > 0 && !found {
		t.Errorf("expected a result with InterceptM=12.5 (57.5 - 45.0), got %+v", results)
	}
}

func TestExtractDrillResults_PercentCopper(t *testing.T) {
	body := `DDH-24-003 returned 20.0m @ 1.8% Cu including 5.0m @ 3.2% Cu`
	results, _ := extractDrillRegex(body)

	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	foundPercent := false
	for _, r := range results {
		if r.Unit == "%" {
			foundPercent = true
		}
	}
	if !foundPercent {
		t.Error("expected at least one result with unit %")
	}
}

func TestExtractDrillResults_NoResults(t *testing.T) {
	body := `The company announced a new mining project in northern Ontario.`
	results, confidence := extractDrillRegex(body)

	if confidence != drillConfidenceNone {
		t.Errorf("confidence = %q, want %q", confidence, drillConfidenceNone)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestExtractDrillResults_PartialSignal(t *testing.T) {
	// Has hole IDs but no grade/intercept pattern
	body := `Drill holes DDH-24-001 and DDH-24-002 were completed. Results are pending.`
	_, confidence := extractDrillRegex(body)

	if confidence != drillConfidencePartial {
		t.Errorf("confidence = %q, want %q", confidence, drillConfidencePartial)
	}
}

func TestClassifyMiningWithDrillExtraction(t *testing.T) {
	body := `Drill hole DDH-24-001 returned 12.5m @ 3.2 g/t Au from 45.0m depth in the Main Zone.`
	result := classifyMiningByRules(
		"Company Reports Drill Results",
		body,
	)

	if result.relevance != miningRelevanceCore {
		t.Errorf("relevance = %q, want core_mining", result.relevance)
	}
	if !result.drillKeywordMatched {
		t.Error("expected drillKeywordMatched=true")
	}
}

func TestClassifyMining_NoDrillKeyword(t *testing.T) {
	result := classifyMiningByRules(
		"Gold Mining Company Expands Operations",
		"The company is expanding its open-pit mining operations.",
	)

	if result.drillKeywordMatched {
		t.Error("expected drillKeywordMatched=false for non-drill article")
	}
}

// Ensure the unused import is satisfied
var _ = domain.DrillResult{}

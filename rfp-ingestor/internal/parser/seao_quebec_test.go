package parser_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/parser"
)

func TestSEAOParser_Parse_GoldenFile(t *testing.T) {
	f, err := os.Open("testdata/seao_sample.json")
	if err != nil {
		t.Fatalf("open golden file: %v", err)
	}
	defer f.Close()

	p := parser.NewSEAOParser()
	docs, rowErrs, err := p.Parse(f)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("Parse row errors: %v", rowErrs)
	}

	if len(docs) != 2 {
		t.Fatalf("expected 2 documents (active tenders only), got %d", len(docs))
	}

	var itDoc, transportDoc bool
	for _, doc := range docs {
		switch doc.RFP.ReferenceNumber {
		case "26-SQ-012":
			itDoc = true
			assertGoldenITServicesTender(t, doc)
		case "1004509":
			transportDoc = true
			assertGoldenTransportTender(t, doc)
		}
	}

	if !itDoc {
		t.Error("missing IT services tender (26-SQ-012)")
	}
	if !transportDoc {
		t.Error("missing transport tender (1004509)")
	}
}

func assertGoldenITServicesTender(t *testing.T, doc domain.RFPDocument) {
	t.Helper()

	assertEqual(t, "Title", "Services professionnels spécialisés en technologie de l'information", doc.Title)
	assertEqual(t, "SourceName", "SEAO", doc.SourceName)
	assertEqual(t, "ContentType", "rfp", doc.ContentType)
	if doc.QualityScore != 75 {
		t.Errorf("QualityScore: expected 75, got %d", doc.QualityScore)
	}
	assertEqual(t, "RFP.OrganizationName", "Santé Québec", doc.RFP.OrganizationName)
	assertEqual(t, "RFP.Province", "qc", doc.RFP.Province)
	assertEqual(t, "RFP.City", "Saint-Hubert", doc.RFP.City)
	assertEqual(t, "RFP.Country", "CA", doc.RFP.Country)
	assertEqual(t, "RFP.UNSPSC", "81110000", doc.RFP.UNSPSC)
	assertEqual(t, "RFP.ProcurementType", "services", doc.RFP.ProcurementType)
	assertEqual(t, "RFP.ClosingDate", "2026-04-15T16:00:00-04:00", doc.RFP.ClosingDate)
	assertEqual(t, "RFP.ExtractionMethod", "json_feed", doc.RFP.ExtractionMethod)
	assertEqual(t, "RFP.BudgetCurrency", "CAD", doc.RFP.BudgetCurrency)
	assertEqual(t, "RFP.TenderStatus", "active", doc.RFP.TenderStatus)

	catSet := toSet(doc.RFP.Categories)
	if _, ok := catSet["it"]; !ok {
		t.Errorf("Categories: expected 'it', got %v", doc.RFP.Categories)
	}
	if _, ok := catSet["it-services"]; !ok {
		t.Errorf("Categories: expected 'it-services', got %v", doc.RFP.Categories)
	}

	topicSet := toSet(doc.Topics)
	if _, ok := topicSet["politics"]; !ok {
		t.Errorf("Topics: expected 'politics', got %v", doc.Topics)
	}
	if _, ok := topicSet["technology"]; !ok {
		t.Errorf("Topics: expected 'technology', got %v", doc.Topics)
	}
}

func assertGoldenTransportTender(t *testing.T, doc domain.RFPDocument) {
	t.Helper()

	assertEqual(t, "RFP.OrganizationName", "Réseau de Transport Métropolitain (EXO)", doc.RFP.OrganizationName)
	assertEqual(t, "RFP.Province", "qc", doc.RFP.Province)
	assertEqual(t, "RFP.City", "Montréal", doc.RFP.City)
}

func TestSEAOParser_SourceName(t *testing.T) {
	p := parser.NewSEAOParser()
	if p.SourceName() != "SEAO" {
		t.Errorf("SourceName: expected 'SEAO', got %q", p.SourceName())
	}
}

func TestSEAOParser_SkipsNonActiveTenders(t *testing.T) {
	jsonData := `{
		"releases": [{
			"ocid": "ocds-test-001",
			"id": "1",
			"date": "2026-01-01T00:00:00Z",
			"tag": ["contract"],
			"buyer": {"name": "Test Org", "id": "OP-1"},
			"tender": {
				"id": "TEST-001",
				"title": "Completed Contract",
				"status": "complete",
				"items": [],
				"tenderPeriod": {}
			},
			"parties": []
		}]
	}`

	p := parser.NewSEAOParser()
	docs, rowErrs, err := p.Parse(stringReader(jsonData))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(rowErrs) != 0 {
		t.Fatalf("Parse row errors: %v", rowErrs)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents for completed contract, got %d", len(docs))
	}
}

func assertEqual(t *testing.T, field, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %q, got %q", field, expected, actual)
	}
}

func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

func stringReader(s string) *strings.Reader {
	return strings.NewReader(s)
}

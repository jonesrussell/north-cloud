package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

func TestToSearchHit_PropagatesRFPData(t *testing.T) {
	budgetMin := 10000.0
	content := &domain.ClassifiedContent{
		ID:          "rfp-001",
		Title:       "Web Redesign RFP",
		ContentType: "rfp",
		RFP: &domain.RFPData{
			OrganizationName: "City of Toronto",
			ClosingDate:      "2026-04-15",
			Province:         "on",
			BudgetMin:        &budgetMin,
		},
	}

	hit := content.ToSearchHit(1.0, nil)

	if hit.RFP == nil {
		t.Fatal("expected RFP data to be propagated, got nil")
	}
	if hit.RFP.OrganizationName != "City of Toronto" {
		t.Errorf("OrganizationName = %q, want %q", hit.RFP.OrganizationName, "City of Toronto")
	}
	if hit.RFP.Province != "on" {
		t.Errorf("Province = %q, want %q", hit.RFP.Province, "on")
	}
}

func TestToSearchHit_NilRFPPassesThrough(t *testing.T) {
	content := &domain.ClassifiedContent{
		ID:          "article-001",
		ContentType: "article",
	}
	hit := content.ToSearchHit(1.0, nil)
	if hit.RFP != nil {
		t.Errorf("expected nil RFP for non-RFP content, got %+v", hit.RFP)
	}
}

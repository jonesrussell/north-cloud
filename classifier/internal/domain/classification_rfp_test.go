package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestContentTypeRFPConstant(t *testing.T) {
	t.Helper()
	assert.Equal(t, "rfp", domain.ContentTypeRFP)
}

func TestRFPResultFields(t *testing.T) {
	t.Helper()
	budgetMin := 50000.0
	budgetMax := 200000.0
	result := &domain.RFPResult{
		ExtractionMethod: "heuristic",
		Title:            "IT Services RFP",
		ReferenceNumber:  "EN578-170432",
		OrganizationName: "Public Services and Procurement Canada",
		ClosingDate:      "2026-04-15T16:00:00Z",
		BudgetMin:        &budgetMin,
		BudgetMax:        &budgetMax,
		BudgetCurrency:   "CAD",
		ProcurementType:  "services",
		NAICSCodes:       []string{"541512"},
		Categories:       []string{"IT", "consulting"},
		Province:         "ON",
		Country:          "CA",
	}
	assert.Equal(t, "heuristic", result.ExtractionMethod)
	assert.Equal(t, "IT Services RFP", result.Title)
	assert.Equal(t, "EN578-170432", result.ReferenceNumber)
	assert.Equal(t, "Public Services and Procurement Canada", result.OrganizationName)
	assert.Equal(t, "2026-04-15T16:00:00Z", result.ClosingDate)
	assert.InDelta(t, 50000.0, *result.BudgetMin, 0.01)
	assert.InDelta(t, 200000.0, *result.BudgetMax, 0.01)
	assert.Equal(t, "CAD", result.BudgetCurrency)
	assert.Equal(t, "services", result.ProcurementType)
	assert.Equal(t, []string{"541512"}, result.NAICSCodes)
	assert.Len(t, result.Categories, 2)
	assert.Equal(t, "ON", result.Province)
	assert.Equal(t, "CA", result.Country)
}

func TestClassificationResultRFPField(t *testing.T) {
	t.Helper()
	result := &domain.ClassificationResult{
		ContentType: domain.ContentTypeRFP,
		RFP: &domain.RFPResult{
			Title: "Test RFP",
		},
	}
	assert.NotNil(t, result.RFP)
	assert.Equal(t, domain.ContentTypeRFP, result.ContentType)
	assert.Equal(t, "Test RFP", result.RFP.Title)
}

func TestClassifiedContentRFPField(t *testing.T) {
	t.Helper()
	content := &domain.ClassifiedContent{
		ContentType: domain.ContentTypeRFP,
		RFP: &domain.RFPResult{
			Title: "Test RFP",
		},
	}
	assert.NotNil(t, content.RFP)
	assert.Equal(t, domain.ContentTypeRFP, content.ContentType)
}

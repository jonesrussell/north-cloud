//nolint:testpackage // Testing internal extractor requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRFPExtractor_NotRFP(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(&mockLogger{})
	raw := &domain.RawContent{ID: "test-1", RawText: "City council met today."}
	result, err := e.Extract(context.Background(), raw, "article", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRFPExtractor_Heuristic(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID: "test-rfp-1",
		RawText: "Reference Number: EN578-170432\n" +
			"Organization: Public Services and Procurement Canada\n" +
			"Closing Date: 2026-04-15\n" +
			"Estimated Value: $500,000\n" +
			"Category: IT Services\n" +
			"Province: Ontario\n",
	}
	result, err := e.Extract(context.Background(), raw, domain.ContentTypeRFP, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "heuristic", result.ExtractionMethod)
	assert.Equal(t, "EN578-170432", result.ReferenceNumber)
	assert.Equal(t, "Public Services and Procurement Canada", result.OrganizationName)
	assert.Equal(t, "2026-04-15", result.ClosingDate)
}

func TestRFPExtractor_TopicGated(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "test-rfp-topic",
		RawText: "Reference Number: ABC-123\nOrganization: City of Toronto\nClosing Date: 2026-05-01\n",
	}
	// Not RFP type, but has rfp topic
	result, err := e.Extract(context.Background(), raw, "article", []string{"rfp"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ABC-123", result.ReferenceNumber)
}

func TestRFPExtractor_NeitherTypeNorTopic(t *testing.T) {
	t.Helper()
	e := NewRFPExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "test-not-rfp",
		RawText: "Reference Number: ABC-123\nOrganization: City of Toronto\n",
	}
	result, err := e.Extract(context.Background(), raw, "article", []string{"crime"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseBudgetValue_Single(t *testing.T) {
	t.Helper()
	budgetMin, budgetMax, currency := parseBudgetValue("$500,000")
	require.NotNil(t, budgetMin)
	require.NotNil(t, budgetMax)
	assert.InDelta(t, 500000.0, *budgetMin, 0.01)
	assert.InDelta(t, 500000.0, *budgetMax, 0.01)
	assert.Equal(t, "CAD", currency)
}

func TestParseBudgetValue_Range(t *testing.T) {
	t.Helper()
	budgetMin, budgetMax, currency := parseBudgetValue("$100,000 - $500,000")
	require.NotNil(t, budgetMin)
	require.NotNil(t, budgetMax)
	assert.InDelta(t, 100000.0, *budgetMin, 0.01)
	assert.InDelta(t, 500000.0, *budgetMax, 0.01)
	assert.Equal(t, "CAD", currency)
}

func TestParseBudgetValue_USD(t *testing.T) {
	t.Helper()
	budgetMin, _, currency := parseBudgetValue("US$250,000")
	require.NotNil(t, budgetMin)
	assert.InDelta(t, 250000.0, *budgetMin, 0.01)
	assert.Equal(t, "USD", currency)
}

func TestParseBudgetValue_Decimal(t *testing.T) {
	t.Helper()
	budgetMin, budgetMax, _ := parseBudgetValue("$500,000.50")
	require.NotNil(t, budgetMin)
	require.NotNil(t, budgetMax)
	assert.InDelta(t, 500000.50, *budgetMin, 0.01)
	assert.InDelta(t, 500000.50, *budgetMax, 0.01)
}

func TestParseBudgetValue_TextOnly(t *testing.T) {
	t.Helper()
	budgetMin, budgetMax, currency := parseBudgetValue("TBD")
	assert.Nil(t, budgetMin)
	assert.Nil(t, budgetMax)
	assert.Empty(t, currency)
}

func TestParseBudgetValue_Empty(t *testing.T) {
	t.Helper()
	budgetMin, budgetMax, currency := parseBudgetValue("")
	assert.Nil(t, budgetMin)
	assert.Nil(t, budgetMax)
	assert.Empty(t, currency)
}

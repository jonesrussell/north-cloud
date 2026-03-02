//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFromRFPKeywords_Match(t *testing.T) {
	t.Helper()
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:    "test-rfp-1",
		Title: "Request for Proposal - IT Infrastructure Modernization",
		RawText: "This request for proposal is for IT infrastructure services. " +
			"The submission deadline is April 15, 2026. " +
			"Proposals must include a detailed scope of work.",
	}

	result := c.classifyFromRFPKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRFP, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromRFPKeywords_NoMatch(t *testing.T) {
	t.Helper()
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-article-1",
		Title:   "City Council Approves New Budget",
		RawText: "The city council met Tuesday to approve the annual operating budget.",
	}

	result := c.classifyFromRFPKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromRFPKeywords_FrenchTender(t *testing.T) {
	t.Helper()
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:    "test-rfp-fr-1",
		Title: "Appel d'offres - Services informatiques",
		RawText: "This call for tenders is for professional services. " +
			"The procurement department requires proposals by March 30.",
	}

	result := c.classifyFromRFPKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRFP, result.Type)
}

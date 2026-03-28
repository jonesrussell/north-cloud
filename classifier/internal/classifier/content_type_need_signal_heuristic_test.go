//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFromNeedSignalKeywords_OutdatedWebsite(t *testing.T) {
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:    "test-need-signal-outdated-1",
		Title: "Sagamok First Nation Community Portal",
		RawText: "This site is powered by Drupal 7. " +
			"We are currently undergoing a site migration to better serve our community. " +
			"Please bear with us during the transition.",
	}

	result := c.classifyFromNeedSignalKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeNeedSignal, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromNeedSignalKeywords_FundingAnnouncement(t *testing.T) {
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:    "test-need-signal-funding-1",
		Title: "Northern Ontario Heritage Fund Announces New Grants",
		RawText: "The funding announcement includes grants for digital transformation " +
			"and website modernization projects across Northern Ontario communities.",
	}

	result := c.classifyFromNeedSignalKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeNeedSignal, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
}

func TestClassifyFromNeedSignalKeywords_JobPosting(t *testing.T) {
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:    "test-need-signal-job-1",
		Title: "Web Developer Needed - Municipality of Espanola",
		RawText: "We are seeking a web developer to help redesign our municipal website. " +
			"The successful candidate will modernize our website redesign initiative.",
	}

	result := c.classifyFromNeedSignalKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeNeedSignal, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
}

func TestClassifyFromNeedSignalKeywords_NoSignal(t *testing.T) {
	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-no-signal-1",
		Title:   "City Council Approves New Budget",
		RawText: "The city council met Tuesday to approve the annual operating budget for the upcoming fiscal year.",
	}

	result := c.classifyFromNeedSignalKeywords(raw)
	assert.Nil(t, result)
}

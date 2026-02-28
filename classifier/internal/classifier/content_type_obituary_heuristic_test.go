//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFromObituaryKeywords_TwoKeywords(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "obit-2kw",
		Title:   "John Smith Obituary",
		RawText: "John Smith passed away peacefully on February 25. He is survived by his wife and two children.",
	}

	result := c.classifyFromObituaryKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeObituary, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromObituaryKeywords_SingleKeyword(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "obit-1kw",
		Title:   "Community Update",
		RawText: "A memorial service will be held for the victims of the flood.",
	}

	result := c.classifyFromObituaryKeywords(raw)
	assert.Nil(t, result, "single keyword match should not classify as obituary")
}

func TestClassifyFromObituaryKeywords_NoSignals(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "obit-none",
		Title:   "Local Sports Recap",
		RawText: "The team won their third consecutive championship this season.",
	}

	result := c.classifyFromObituaryKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromObituaryKeywords_CrimeSuppression(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	tests := []struct {
		name    string
		title   string
		rawText string
	}{
		{
			name:    "police said suppresses obituary",
			title:   "Man Found Dead",
			rawText: "The victim passed away after the incident. Police said they are investigating the circumstances. He is survived by his family.",
		},
		{
			name:    "charged with suppresses obituary",
			title:   "Death Investigation",
			rawText: "The person passed away in hospital. The suspect was charged with assault. Condolences poured in.",
		},
		{
			name:    "arrested suppresses obituary",
			title:   "Tragedy Strikes",
			rawText: "The elderly man passed away after the altercation. A suspect was arrested at the scene. The funeral will be held Saturday.",
		},
		{
			name:    "under investigation suppresses obituary",
			title:   "Death Under Investigation",
			rawText: "He passed away suddenly. The death is under investigation by detectives. Survived by his wife.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:      "obit-crime-" + tt.name,
				Title:   tt.title,
				RawText: tt.rawText,
			}
			result := c.classifyFromObituaryKeywords(raw)
			assert.Nil(t, result, "crime keyword should suppress obituary classification")
		})
	}
}

func TestClassifyFromObituaryKeywords_CaseInsensitive(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "obit-case",
		Title:   "IN LOVING MEMORY of Jane Doe",
		RawText: "She PASSED AWAY on February 20. SURVIVED BY her three children.",
	}

	result := c.classifyFromObituaryKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeObituary, result.Type)
}

// Cross-strategy conflict: crime article with "passed away" + "police said" — NOT obituary.
func TestCrimeArticleNotObituaryViaFullCascade(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:    "crime-not-obit",
		URL:   "https://example.com/news/homicide-investigation",
		Title: "Man Dies After Assault",
		RawText: "The victim passed away in hospital. " +
			"Police said a suspect has been arrested. " +
			"He is survived by his wife and children. " +
			string(make([]byte, 500)),
		WordCount:       350,
		MetaDescription: "Police investigating after man dies following assault",
		PublishedDate:   &publishedDate,
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.NotEqual(t, domain.ContentTypeObituary, result.Type,
		"crime article with 'passed away' + 'police said' must NOT be classified as obituary")
}

// Cross-strategy conflict: obituary wins over article heuristics.
func TestObituaryWinsOverArticleHeuristic(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:    "obit-vs-article",
		URL:   "https://example.com/obituaries/john-smith",
		Title: "John Smith - Obituary",
		RawText: "John Smith passed away peacefully at home. " +
			"He is survived by his loving wife Mary. " +
			"A celebration of life will be held. " +
			string(make([]byte, 500)),
		WordCount:       300,
		MetaDescription: "Obituary for John Smith",
		PublishedDate:   &publishedDate,
		OGType:          "article",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeObituary, result.Type, "obituary heuristic should win over article")
	assert.Equal(t, "keyword_heuristic", result.Method)
}

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

func TestClassifyFromJobKeywords_TwoKeywords(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "job-2kw",
		Title:   "Software Developer Position",
		RawText: "Job description: We are looking for a developer. Requirements include Go experience. Apply now.",
	}

	result := c.classifyFromJobKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeJob, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromJobKeywords_SingleKeyword(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "job-1kw",
		Title:   "Company News",
		RawText: "The salary for this position has been increased.",
	}

	result := c.classifyFromJobKeywords(raw)
	assert.Nil(t, result, "single keyword match should not classify as job")
}

func TestClassifyFromJobKeywords_NoSignals(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "job-none",
		Title:   "Local News Update",
		RawText: "The mayor announced new park improvements.",
	}

	result := c.classifyFromJobKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromJobKeywords_CaseInsensitive(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "job-case",
		Title:   "APPLY NOW - Full-Time Position",
		RawText: "RESPONSIBILITIES include managing the team.",
	}

	result := c.classifyFromJobKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeJob, result.Type)
}

// Cross-strategy conflict: job page with og:type="website" — job wins over OG.
func TestJobWinsOverOGWebsite(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "job-vs-og",
		URL:     "https://example.com/careers/developer",
		Title:   "Software Developer",
		RawText: "Job description: Build scalable systems. Requirements: 3+ years experience. Apply now with your resume.",
		OGType:  "website",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeJob, result.Type, "job heuristic should win over OG website")
	assert.Equal(t, "keyword_heuristic", result.Method)
}

// Cross-strategy conflict: job page with og:type="article" — job wins over demoted OG.
func TestJobWinsOverOGArticle(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:              "job-vs-og-article",
		URL:             "https://example.com/jobs/plumber",
		Title:           "Plumber Needed",
		RawText:         "Full-time position available. Salary competitive. Requirements: licensed plumber. " + string(make([]byte, 500)),
		WordCount:       300,
		MetaDescription: "We are hiring a licensed plumber",
		PublishedDate:   &publishedDate,
		OGType:          "article",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeJob, result.Type, "job heuristic should win over demoted OG article")
}

// URL exclusion: /classifieds index page is still excluded, but /classifieds/job-listing passes through.
func TestClassifiedsURLExclusionFix(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	tests := []struct {
		name         string
		url          string
		expectedType string
	}{
		{
			name:         "classifieds index page excluded",
			url:          "https://example.com/classifieds",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "classifieds index with trailing slash excluded",
			url:          "https://example.com/classifieds/",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "classifieds subpath with job keywords passes through as job",
			url:          "https://example.com/classifieds/plumber-needed",
			expectedType: domain.ContentTypeJob,
		},
		{
			name:         "jobs index page excluded",
			url:          "https://example.com/jobs",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "careers index page excluded",
			url:          "https://example.com/careers",
			expectedType: domain.ContentTypePage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:      "test-" + tt.name,
				URL:     tt.url,
				Title:   "Test Page",
				RawText: "Full-time position available. Salary competitive. Requirements: 3 years experience.",
			}

			result, err := c.Classify(context.Background(), raw)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, result.Type, "URL: %s", tt.url)
		})
	}
}

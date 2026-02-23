//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobExtractor_SchemaOrgFullFields(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "job-1",
		Title: "Senior Go Developer",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "JobPosting",
  "title": "Senior Go Developer",
  "hiringOrganization": {"@type": "Organization", "name": "Acme Corp"},
  "jobLocation": {
    "@type": "Place",
    "address": {
      "@type": "PostalAddress",
      "addressLocality": "Toronto",
      "addressRegion": "ON"
    }
  },
  "baseSalary": {
    "@type": "MonetaryAmount",
    "currency": "CAD",
    "value": {
      "@type": "QuantitativeValue",
      "minValue": 120000,
      "maxValue": 160000
    }
  },
  "employmentType": "FULL_TIME",
  "datePosted": "2026-02-01",
  "validThrough": "2026-03-01",
  "description": "We are looking for a senior Go developer.",
  "industry": "Technology",
  "qualifications": "5+ years Go experience",
  "jobBenefits": "Health insurance, remote work"
}
</script>
</head><body></body></html>`,
		RawText: "Senior Go Developer job posting",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeJob, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Equal(t, "Senior Go Developer", result.Title)
	assert.Equal(t, "Acme Corp", result.Company)
	assert.Equal(t, "Toronto, ON", result.Location)

	require.NotNil(t, result.SalaryMin)
	assert.InDelta(t, 120000.0, *result.SalaryMin, 0.01)

	require.NotNil(t, result.SalaryMax)
	assert.InDelta(t, 160000.0, *result.SalaryMax, 0.01)

	assert.Equal(t, "CAD", result.SalaryCurrency)
	assert.Equal(t, "full_time", result.EmploymentType)
	assert.Equal(t, "2026-02-01", result.PostedDate)
	assert.Equal(t, "2026-03-01", result.ExpiresDate)
	assert.Equal(t, "We are looking for a senior Go developer.", result.Description)
	assert.Equal(t, "Technology", result.Industry)
	assert.Equal(t, "5+ years Go experience", result.Qualifications)
	assert.Equal(t, "Health insurance, remote work", result.Benefits)
}

func TestJobExtractor_NotApplicable_ArticleContentType(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "job-2",
		Title:   "Breaking News Article",
		RawHTML: `<html><body><p>This is a news article.</p></body></html>`,
		RawText: "This is a news article about local events.",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"crime", "local_news"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestJobExtractor_HeuristicFallback(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "job-3",
		Title:   "Software Engineer Position",
		RawHTML: `<html><body><p>No JSON-LD here</p></body></html>`,
		RawText: `Software Engineer Position

Company: TechStartup Inc
Location: Vancouver, BC

We are hiring a software engineer to join our team.

Requirements:
Must have 3 years experience with Python.
Must have experience with AWS.

Qualifications:
BSc in Computer Science or equivalent.
Strong communication skills.`,
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"jobs", "technology"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "heuristic", result.ExtractionMethod)
	assert.Equal(t, "TechStartup Inc", result.Company)
	assert.Equal(t, "Vancouver, BC", result.Location)
	assert.Contains(t, result.Qualifications, "Must have 3 years experience with Python")
}

func TestJobExtractor_EmploymentTypeNormalization(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "FULL_TIME", input: "FULL_TIME", expected: "full_time"},
		{name: "PART_TIME", input: "PART_TIME", expected: "part_time"},
		{name: "CONTRACT", input: "CONTRACT", expected: "contract"},
		{name: "TEMPORARY", input: "TEMPORARY", expected: "temporary"},
		{name: "INTERN", input: "INTERN", expected: "internship"},
		{name: "INTERNSHIP", input: "INTERNSHIP", expected: "internship"},
		{name: "lowercase_passthrough", input: "full_time", expected: "full_time"},
		{name: "unknown_passthrough", input: "VOLUNTEER", expected: "volunteer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:    "job-norm-" + tt.name,
				Title: "Test Job",
				RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "JobPosting",
  "title": "Test Job",
  "employmentType": "` + tt.input + `"
}
</script>
</head><body></body></html>`,
				RawText: "Test Job posting",
			}

			result, err := extractor.Extract(ctx, raw, domain.ContentTypeJob, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result.EmploymentType)
		})
	}
}

func TestJobExtractor_TopicGating_JobsTopic(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "job-5",
		Title: "Help Wanted: Barista",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "JobPosting",
  "title": "Barista",
  "hiringOrganization": {"@type": "Organization", "name": "Coffee Shop"}
}
</script>
</head><body></body></html>`,
		RawText: "Barista job posting",
	}

	// Content type is article but topics include "jobs" — should still extract.
	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"jobs"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Equal(t, "Barista", result.Title)
	assert.Equal(t, "Coffee Shop", result.Company)
}

func TestJobExtractor_SchemaOrgLocationCityOnly(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "job-6",
		Title: "Data Analyst",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "JobPosting",
  "title": "Data Analyst",
  "jobLocation": {
    "@type": "Place",
    "address": {
      "@type": "PostalAddress",
      "addressLocality": "Montreal"
    }
  }
}
</script>
</head><body></body></html>`,
		RawText: "Data Analyst job",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeJob, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Montreal", result.Location)
}

func TestJobExtractor_HeuristicReturnsNilWhenNothingFound(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "job-7",
		Title:   "Random Content",
		RawHTML: `<html><body><p>No structured data</p></body></html>`,
		RawText: "Just some random text with no job patterns at all.",
	}

	// Topics contain "jobs" but no Schema.org and no heuristic patterns.
	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"jobs"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestJobExtractor_SchemaOrgLocationAndSalaryEdgeCases(t *testing.T) {
	t.Helper()

	extractor := NewJobExtractor(&mockLogger{})
	ctx := context.Background()

	t.Run("missing jobLocation", func(t *testing.T) {
		raw := &domain.RawContent{
			ID: "no-loc", Title: "Job",
			RawHTML: `<html><head><script type="application/ld+json">
{"@context":"https://schema.org","@type":"JobPosting","title":"Developer","hiringOrganization":{"name":"Acme"}}
</script></head><body></body></html>`,
			RawText: "Developer job",
		}
		result, err := extractor.Extract(ctx, raw, domain.ContentTypeJob, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Location)
	})

	t.Run("missing baseSalary", func(t *testing.T) {
		raw := &domain.RawContent{
			ID: "no-salary", Title: "Job",
			RawHTML: `<html><head><script type="application/ld+json">
{"@context":"https://schema.org","@type":"JobPosting","title":"Volunteer","hiringOrganization":{"name":"NGO"}}
</script></head><body></body></html>`,
			RawText: "Volunteer",
		}
		result, err := extractor.Extract(ctx, raw, domain.ContentTypeJob, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Nil(t, result.SalaryMin)
		assert.Nil(t, result.SalaryMax)
	})
}

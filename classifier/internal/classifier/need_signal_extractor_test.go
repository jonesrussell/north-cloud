//nolint:testpackage // Testing internal extractor requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeedSignalExtractor_Extract_OutdatedWebsite(t *testing.T) {
	e := NewNeedSignalExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID:    "test-ns-1",
		Title: "City of Thunder Bay - Website Redesign Project",
		RawText: "The City of Thunder Bay is seeking proposals for a complete website redesign. " +
			"The current site runs on Drupal 7, which has reached end of life. " +
			"The legacy website must be migrated to a modern platform. " +
			"For inquiries, contact jsmith@thunderbay.ca.",
		URL: "https://thunderbay.ca/redesign",
	}

	result, err := e.Extract(context.Background(), raw, domain.ContentTypeNeedSignal, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SignalTypeOutdatedWebsite, result.SignalType)
	assert.Contains(t, result.OrganizationName, "Thunder Bay")
	assert.Equal(t, "jsmith@thunderbay.ca", result.ContactEmail)
	assert.NotEmpty(t, result.Keywords)
	assert.InDelta(t, needSignalConfidence, result.Confidence, 0.01)
}

func TestNeedSignalExtractor_Extract_WrongContentType(t *testing.T) {
	e := NewNeedSignalExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "test-ns-2",
		Title:   "Some Article",
		RawText: "This is an article about Drupal 7 migration.",
	}

	result, err := e.Extract(context.Background(), raw, "article", nil)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestNeedSignalExtractor_Extract_FundingWin(t *testing.T) {
	e := NewNeedSignalExtractor(&mockLogger{})
	raw := &domain.RawContent{
		ID:    "test-ns-3",
		Title: "Sagamok Anishnawbek receives digital capacity grant",
		RawText: "Sagamok Anishnawbek has been awarded grant funding for digital capacity building. " +
			"The funding announcement confirms infrastructure funding to support " +
			"digital transformation initiatives across the community.",
		URL: "https://sagamok.ca/news/grant",
	}

	result, err := e.Extract(context.Background(), raw, domain.ContentTypeNeedSignal, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SignalTypeFundingWin, result.SignalType)
	assert.Contains(t, result.OrganizationName, "Sagamok Anishnawbek")
	assert.NotEmpty(t, result.Keywords)
}

func TestNeedSignalExtractor_ExtractOrgName_Delimiters(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{name: "dash_delimiter", title: "City of Toronto - Website RFP", expected: "City of Toronto"},
		{name: "pipe_delimiter", title: "Sagamok | Digital Grant", expected: "Sagamok"},
		{name: "colon_delimiter", title: "Thunder Bay: New Website", expected: "Thunder Bay"},
		{name: "announces_delimiter", title: "Wikwemikong announces new portal", expected: "Wikwemikong"},
		{name: "no_delimiter", title: "Complete Title Without Delimiter", expected: "Complete Title Without Delimiter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractOrgName(tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNeedSignalExtractor_ExtractContactEmail(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "found", text: "Contact us at info@example.ca for details", expected: "info@example.ca"},
		{name: "not_found", text: "No email address here", expected: ""},
		{name: "multiple", text: "Email a@b.com or c@d.com", expected: "a@b.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContactEmail(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNeedSignalExtractor_DetectSignalType(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "outdated_website",
			text:     "drupal 7 legacy website site redesign",
			expected: SignalTypeOutdatedWebsite,
		},
		{
			name:     "funding_win",
			text:     "funding announcement grant funding receives funding awarded grant",
			expected: SignalTypeFundingWin,
		},
		{
			name:     "job_posting",
			text:     "web developer frontend developer full stack developer",
			expected: SignalTypeJobPosting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectSignalType(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

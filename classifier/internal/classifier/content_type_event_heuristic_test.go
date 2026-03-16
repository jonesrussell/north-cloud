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

func TestClassifyFromEventKeywords_TwoKeywords(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-keywords",
		Title:   "Annual Tech Conference",
		RawText: "Register now for the biggest event of the year. Tickets available at the door.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromEventKeywords_SingleKeyword_NoMatch(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-single",
		Title:   "Conference Info",
		RawText: "The venue is downtown. No other event signals here.",
	}

	result := c.classifyFromEventKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromEventKeywords_DateLocation_VenuePhrase(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-date-venue",
		Title:   "Spring Gala",
		RawText: "Join us on March 15, 2026 at the Community Hall for an evening of music.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromEventKeywords_DateLocation_StreetAddress(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-date-street",
		Title:   "Open House",
		RawText: "Come visit us on January 20, 2027 at 123 Main Street for a tour.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
}

func TestClassifyFromEventKeywords_DateOnly_NoLocation(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-date-no-location",
		Title:   "Article about history",
		RawText: "Something happened on July 4, 1776 that changed the world.",
	}

	result := c.classifyFromEventKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromEventKeywords_NoSignals(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-no-signals",
		Title:   "Regular News Article",
		RawText: "The mayor announced a new policy for the city council.",
	}

	result := c.classifyFromEventKeywords(raw)
	assert.Nil(t, result)
}

// TestEventWinsOverArticleHeuristic verifies that an event page that also
// satisfies article heuristics (date + 200+ words) is classified as event.
func TestEventWinsOverArticleHeuristic(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	publishedDate := time.Now()

	raw := &domain.RawContent{
		ID:    "test-event-vs-article",
		URL:   "https://example.com/events/gala",
		Title: "Annual Gala Night",
		RawText: "Register now for the annual gala. Tickets available online. " +
			"This is a lengthy description of the event with lots of words to push the word count " +
			"above the article threshold. The event will feature keynote speakers and music.",
		WordCount:       250,
		MetaDescription: "Annual gala event",
		PublishedDate:   &publishedDate,
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

// TestSchemaOrgEventDetection verifies Schema.org Event JSON-LD is detected
// with confidence 1.0.
func TestSchemaOrgEventDetection(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID: "test-schema-event",
		RawHTML: `<html><head>
		<script type="application/ld+json">{"@type": "Event", "name": "Tech Conference 2026"}</script>
		</head><body></body></html>`,
		OGType: "article",
		Title:  "Tech Conference 2026",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
	assert.Equal(t, "schema_org", result.Method)
	assert.InDelta(t, schemaOrgConfidence, result.Confidence, 0.001)
}

// TestDateLocationHeuristic_FullCascade verifies the date-location heuristic
// fires through the full Classify cascade when no keywords match.
func TestDateLocationHeuristic_FullCascade(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-date-loc-cascade",
		URL:     "https://example.com/gathering",
		Title:   "Community Gathering",
		RawText: "Join us on September 10, 2026 at the Civic Centre for a community discussion.",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
	assert.Equal(t, "keyword_heuristic", result.Method)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
}

func TestHasLocationSignal(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{"venue phrase", "meet at the convention center", true},
		{"venue label", "venue: downtown arena", true},
		{"street address", "located at 42 Oak Avenue downtown", true},
		{"drive address", "visit us at 100 Sunset Drive", true},
		{"no signal", "a regular sentence with no location", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := hasLocationSignal(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyFromEventKeywords_CaseInsensitive(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-case",
		Title:   "REGISTER NOW for the GALA",
		RawText: "DOORS OPEN at 7pm. Come early for the best seats.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
}

func TestClassifyFromEventKeywords_EventReport_ScheduledFor(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-report-scheduled",
		Title:   "Annual Music Festival Returns to Sudbury",
		RawText: "The popular music festival is scheduled for next weekend at the waterfront park.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeArticle, result.Type)
	assert.Equal(t, domain.ContentSubtypeEventReport, result.Subtype)
	assert.Equal(t, "event_report_heuristic", result.Method)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
}

func TestClassifyFromEventKeywords_EventReport_WillTakePlace(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-event-report-takeplace",
		Title:   "Protest March Planned for Downtown",
		RawText: "The demonstration will take place Saturday morning starting at city hall.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeArticle, result.Type)
	assert.Equal(t, domain.ContentSubtypeEventReport, result.Subtype)
}

func TestClassifyFromEventKeywords_EventReport_DoesNotOverrideEvent(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	// This has 2+ event keywords, so it should be classified as event, not event_report
	raw := &domain.RawContent{
		ID:      "test-event-not-report",
		Title:   "Register Now for the Festival",
		RawText: "Tickets available at the door. The event is scheduled for Saturday.",
	}

	result := c.classifyFromEventKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeEvent, result.Type)
}

func TestClassifyFromEventKeywords_EventReport_NoSignal_ReturnsNil(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:      "test-no-event-report",
		Title:   "City Council Approves New Budget",
		RawText: "The council voted unanimously to approve the annual budget for the city.",
	}

	result := c.classifyFromEventKeywords(raw)
	assert.Nil(t, result)
}

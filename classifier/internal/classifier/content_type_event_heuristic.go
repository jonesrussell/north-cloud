package classifier

import (
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Event keyword heuristic constants.
const (
	keywordHeuristicConfidence = 0.80
	minKeywordMatches          = 2
)

// eventKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page describes an upcoming or past event.
// Requiring 2+ matches avoids false positives from pages that
// incidentally mention one term.
var eventKeywords = []string{
	"register now",
	"tickets available",
	"event date",
	"venue",
	"admission",
	"doors open",
	"rsvp",
	"keynote speaker",
	"registration deadline",
}

// futureDatePattern matches full month-name date strings like "January 5, 2026"
// or "March 12 2027". The pattern is intentionally broad (any year) because
// historical event pages are still event-type content.
var futureDatePattern = regexp.MustCompile(
	`(?i)\b(January|February|March|April|May|June|July|August|September|` +
		`October|November|December)\s+\d{1,2},?\s+\d{4}\b`,
)

// locationSignalPhrases are short phrases that suggest a physical venue
// or location when paired with a date.
var locationSignalPhrases = []string{
	"at the",
	"venue:",
}

// streetAddressPattern matches common North American street address
// formats like "123 Main Street" or "42 Oak Ave".
var streetAddressPattern = regexp.MustCompile(
	`(?i)\d+\s+\w+\s+(?:Street|St|Avenue|Ave|Road|Rd|Drive|Dr|Boulevard|Blvd)\b`,
)

// classifyFromEventKeywords checks title + raw_text for event-related
// keywords. Returns ContentTypeEvent with confidence 0.80 when at least
// 2 keyword matches are found, or when a date-location heuristic fires.
// Returns nil if no event signal is detected.
func (c *ContentTypeClassifier) classifyFromEventKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	// Path 1: keyword counting
	if result := c.matchEventKeywords(raw, combinedText); result != nil {
		return result
	}

	// Path 2: date + location heuristic
	return c.matchDateLocation(raw, combinedText)
}

// matchEventKeywords counts event keyword hits and returns a result
// when at least minKeywordMatches are found.
func (c *ContentTypeClassifier) matchEventKeywords(
	raw *domain.RawContent, text string,
) *ContentTypeResult {
	matches := 0

	for _, kw := range eventKeywords {
		if strings.Contains(text, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("Event detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeEvent,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "Event keywords detected in content",
			}
		}
	}

	return nil
}

// matchDateLocation checks for a date pattern combined with a location
// signal (phrase or street address). This provides an alternative
// detection path when explicit event keywords are absent.
func (c *ContentTypeClassifier) matchDateLocation(
	raw *domain.RawContent, text string,
) *ContentTypeResult {
	if !futureDatePattern.MatchString(text) {
		return nil
	}

	if !hasLocationSignal(text) {
		return nil
	}

	c.logger.Debug("Event detected via date-location heuristic",
		infralogger.String("content_id", raw.ID),
	)

	return &ContentTypeResult{
		Type:       domain.ContentTypeEvent,
		Confidence: keywordHeuristicConfidence,
		Method:     "keyword_heuristic",
		Reason:     "Future date pattern with location signal detected",
	}
}

// hasLocationSignal checks whether the text contains a venue phrase
// or a street-address-like pattern.
func hasLocationSignal(text string) bool {
	for _, phrase := range locationSignalPhrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}

	return streetAddressPattern.MatchString(text)
}

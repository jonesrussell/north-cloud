package rawcontent

import "strings"

// Page type constants classify the structural nature of a crawled page.
const (
	pageTypeArticle = "article"
	pageTypeStub    = "stub"
	pageTypeListing = "listing"
	pageTypeOther   = "other"
)

// Word/link thresholds for page type classification.
const (
	articleMinWordCount   = 200
	stubMaxWordCount      = 50
	listingMinLinkCount   = 20
	listingMaxWordPerLink = 10
)

// Scoring weights for article signals.
const (
	scoreJSONLDArticle       = 5
	scoreOGTypeArticle       = 3
	scoreDetectedTypeArticle = 3
	scoreArticleTag          = 2
	scoreDateTime            = 1
	scoreWordCountTitle      = 4
	articleScoreThreshold    = 4
)

// jsonLDArticleTypes are schema.org types that indicate article content.
var jsonLDArticleTypes = map[string]bool{
	"article":     true,
	"newsarticle": true,
	"blogposting": true,
	"reportage":   true,
}

// pageTypeSignals holds all signals used to classify a page's structural type.
type pageTypeSignals struct {
	title               string
	wordCount           int
	linkCount           int
	ogType              string
	detectedContentType string
	jsonLDType          string
	articleTagCount     int
	hasDateTime         bool
	hasSignInText       bool
}

// classifyPageType assigns a page type from a scored set of structural signals.
// Sign-in pages are always "other". Otherwise article, listing, stub, or other
// is chosen in order of decreasing specificity.
func classifyPageType(s pageTypeSignals) string {
	if s.hasSignInText {
		return pageTypeOther
	}

	if computeArticleScore(s) >= articleScoreThreshold {
		return pageTypeArticle
	}

	if isListingPage(s.linkCount, s.wordCount) {
		return pageTypeListing
	}

	if s.title != "" && s.wordCount < stubMaxWordCount {
		return pageTypeStub
	}

	return pageTypeOther
}

// computeArticleScore sums weighted signals that indicate article content.
func computeArticleScore(s pageTypeSignals) int {
	score := 0

	if jsonLDArticleTypes[strings.ToLower(s.jsonLDType)] {
		score += scoreJSONLDArticle
	}

	if strings.EqualFold(s.ogType, "article") {
		score += scoreOGTypeArticle
	}

	if strings.EqualFold(s.detectedContentType, "article") {
		score += scoreDetectedTypeArticle
	}

	if s.articleTagCount > 0 {
		score += scoreArticleTag
	}

	if s.hasDateTime {
		score += scoreDateTime
	}

	if s.title != "" && s.wordCount >= articleMinWordCount {
		score += scoreWordCountTitle
	}

	return score
}

// isListingPage returns true when links are dense relative to word count.
func isListingPage(linkCount, wordCount int) bool {
	return linkCount >= listingMinLinkCount &&
		(wordCount == 0 || wordCount/linkCount < listingMaxWordPerLink)
}

// extractHTMLSignals scans rawHTML for structural signals without full parsing.
// Returns the number of <article> elements, whether a <time datetime= attribute
// is present, and whether sign-in language is detected.
func extractHTMLSignals(rawHTML string) (articleTagCount int, hasDateTime, hasSignInText bool) {
	lower := strings.ToLower(rawHTML)
	articleTagCount = strings.Count(lower, "<article")
	hasDateTime = strings.Contains(lower, "<time datetime")
	hasSignInText = strings.Contains(lower, "sign in") ||
		strings.Contains(lower, "log in") ||
		strings.Contains(lower, "sign-in")
	return
}

// extractJSONLDType returns the @type value from a JSON-LD data map, or "".
func extractJSONLDType(data map[string]any) string {
	if data == nil {
		return ""
	}
	v, ok := data["@type"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

package testcrawl

import "strings"

// pageType constants classify the structural nature of a page.
const (
	PageTypeArticle = "article"
	PageTypeStub    = "stub"
	PageTypeListing = "listing"
	PageTypeOther   = "other"
)

// Word/link thresholds for page type classification (mirrors crawler).
const (
	articleMinWordCount   = 200
	stubMaxWordCount      = 50
	listingMinLinkCount   = 20
	listingMaxWordPerLink = 10
)

// Scoring weights for article signals (mirrors crawler).
const (
	scoreJSONLDArticle    = 5
	scoreOGTypeArticle    = 3
	scoreArticleTag       = 2
	scoreDateTime         = 1
	scoreWordCountTitle   = 4
	articleScoreThreshold = 4
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
	title           string
	wordCount       int
	linkCount       int
	ogType          string
	jsonLDType      string
	articleTagCount int
	hasDateTime     bool
	hasSignInText   bool
}

// ClassifyPageType assigns a page type from scored structural signals.
func ClassifyPageType(s pageTypeSignals) string {
	if s.hasSignInText {
		return PageTypeOther
	}
	if computeArticleScore(s) >= articleScoreThreshold {
		return PageTypeArticle
	}
	if isListingPage(s.linkCount, s.wordCount) {
		return PageTypeListing
	}
	if s.title != "" && s.wordCount < stubMaxWordCount {
		return PageTypeStub
	}
	return PageTypeOther
}

func computeArticleScore(s pageTypeSignals) int {
	score := 0
	if jsonLDArticleTypes[strings.ToLower(s.jsonLDType)] {
		score += scoreJSONLDArticle
	}
	if strings.EqualFold(s.ogType, "article") {
		score += scoreOGTypeArticle
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

func isListingPage(linkCount, wordCount int) bool {
	return linkCount >= listingMinLinkCount &&
		(wordCount == 0 || wordCount/linkCount < listingMaxWordPerLink)
}

// extractHTMLSignals scans rawHTML for structural signals without full re-parsing.
func extractHTMLSignals(rawHTML string) (articleTagCount int, hasDateTime, hasSignInText bool) {
	lower := strings.ToLower(rawHTML)
	articleTagCount = strings.Count(lower, "<article")
	hasDateTime = strings.Contains(lower, "<time datetime")
	hasSignInText = strings.Contains(lower, "sign in") ||
		strings.Contains(lower, "log in") ||
		strings.Contains(lower, "sign-in")
	return
}

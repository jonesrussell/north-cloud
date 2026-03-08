package rawcontent

// Page type constants classify the structural nature of a crawled page.
const (
	pageTypeArticle = "article"
	pageTypeStub    = "stub"
	pageTypeListing = "listing"
	pageTypeOther   = "other"
)

// Thresholds for page type classification heuristics.
const (
	articleMinWordCount   = 200
	stubMaxWordCount      = 50
	listingMinLinkCount   = 20
	listingMaxWordPerLink = 10
)

// classifyPageType assigns a page type based on title presence, word count, and link density.
func classifyPageType(title string, wordCount, linkCount int) string {
	hasTitle := title != ""

	if hasTitle && wordCount >= articleMinWordCount {
		return pageTypeArticle
	}

	if hasTitle && wordCount < stubMaxWordCount {
		return pageTypeStub
	}

	if linkCount >= listingMinLinkCount && (wordCount == 0 || wordCount/linkCount < listingMaxWordPerLink) {
		return pageTypeListing
	}

	return pageTypeOther
}

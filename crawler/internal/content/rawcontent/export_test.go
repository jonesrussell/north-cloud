package rawcontent

// Test exports for internal functions

var NormalizeContextField = normalizeContextField

var NormalizeJSONLDObject = normalizeJSONLDObject

var NormalizeImageField = normalizeImageField

var NormalizeObjectToName = normalizeObjectToName

var NormalizeEntityToURL = normalizeEntityToURL

var NormalizeToString = normalizeToString

var ExtractJSONLDHeadline = extractJSONLDHeadline

// LookupTemplate exports lookupTemplate for testing.
var LookupTemplate = lookupTemplate

// LookupTemplateByName exports lookupTemplateByName for testing.
var LookupTemplateByName = lookupTemplateByName

// DetectTemplateByHTML exports detectTemplateByHTML for testing.
var DetectTemplateByHTML = detectTemplateByHTML

// TemplateRegistry exports templateRegistry for testing.
var TemplateRegistry = templateRegistry

// ClassifyPageType exports classifyPageType for testing.
var ClassifyPageType = classifyPageType

// ExtractHTMLSignals exports extractHTMLSignals for testing.
var ExtractHTMLSignals = extractHTMLSignals

// Exported page type constants for testing.
const (
	PageTypeArticle = pageTypeArticle
	PageTypeStub    = pageTypeStub
	PageTypeListing = pageTypeListing
	PageTypeOther   = pageTypeOther
)

// MakeSignals constructs a pageTypeSignals for use in external test packages.
// Only the fields needed for the test case need to be specified; zero values
// are safe defaults (empty strings, false, 0).
func MakeSignals(
	title string,
	wordCount, linkCount int,
	ogType, detectedContentType, jsonLDType string,
	articleTagCount int,
	hasDateTime, hasSignInText bool,
) pageTypeSignals {
	return pageTypeSignals{
		title:               title,
		wordCount:           wordCount,
		linkCount:           linkCount,
		ogType:              ogType,
		detectedContentType: detectedContentType,
		jsonLDType:          jsonLDType,
		articleTagCount:     articleTagCount,
		hasDateTime:         hasDateTime,
		hasSignInText:       hasSignInText,
	}
}

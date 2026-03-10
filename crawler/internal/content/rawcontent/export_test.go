package rawcontent

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

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

// ResolveTemplateForTest exposes resolveTemplate for testing via a no-op service.
// templateHint is optional; pass nil to skip hint lookup.
func ResolveTemplateForTest(templateHint *string, sourceURL, rawHTML string) (tmpl *CMSTemplate, name string) {
	svc := &RawContentService{logger: infralogger.NewNop()}
	cfg := &sources.Config{TemplateHint: templateHint}
	return svc.resolveTemplate(cfg, sourceURL, rawHTML)
}

// ExtractByTextDensity exports extractByTextDensity for testing.
var ExtractByTextDensity = extractByTextDensity

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

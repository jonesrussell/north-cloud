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

// TemplateRegistry exports templateRegistry for testing.
var TemplateRegistry = templateRegistry

// ClassifyPageType exports classifyPageType for testing.
var ClassifyPageType = classifyPageType

// Exported page type constants for testing.
const (
	PageTypeArticle = pageTypeArticle
	PageTypeStub    = pageTypeStub
	PageTypeListing = pageTypeListing
	PageTypeOther   = pageTypeOther
)

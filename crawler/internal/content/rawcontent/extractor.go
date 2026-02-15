// Package rawcontent provides functionality for extracting raw content from any HTML page
// without making assumptions about content type. The classifier will handle type detection.
package rawcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"encoding/json"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// JSON-LD schema type constants for content extraction
const (
	jsonldTypeNewsArticle         = "NewsArticle"
	jsonldTypeArticle             = "Article"
	jsonldTypeBlogPosting         = "BlogPosting"
	jsonldTypePressRelease        = "PressRelease"
	jsonldTypeEvent               = "Event"
	jsonldTypeSpecialAnnouncement = "SpecialAnnouncement"
	jsonldTypeReport              = "Report"
)

// ArticleMeta contains article-specific metadata
type ArticleMeta struct {
	Section     string
	Opinion     bool
	ContentTier string
}

// TwitterMeta contains Twitter card metadata
type TwitterMeta struct {
	Card string
	Site string
}

// ExtendedOG contains extended Open Graph metadata
type ExtendedOG struct {
	ImageWidth  int
	ImageHeight int
	SiteName    string
}

// RawContentData represents extracted raw content from any HTML page
type RawContentData struct {
	ID                 string
	URL                string
	Title              string
	RawText            string
	RawHTML            string
	MetaDescription    string
	MetaKeywords       string
	OGType             string
	OGTitle            string
	OGDescription      string
	OGImage            string
	OGURL              string
	CanonicalURL       string
	Author             string
	PublishedDate      *time.Time
	ArticleSection     string
	ArticleOpinion     bool
	ArticleContentTier string
	TwitterCard        string
	TwitterSite        string
	OGImageWidth       int
	OGImageHeight      int
	OGSiteName         string
	JSONLDData         map[string]any
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ExtractRawContent extracts raw content from any HTML element without type assumptions.
// Uses available selectors if provided, but falls back to generic extraction strategies.
func ExtractRawContent(
	e *colly.HTMLElement,
	sourceURL, titleSelector, bodySelector, containerSelector string,
	excludeSelectors []string,
) *RawContentData {
	data := &RawContentData{
		URL:       sourceURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract title - try selector first, then OG, then fallback
	data.Title = extractTitle(e, titleSelector)

	// Extract raw HTML - preserve original HTML for classifier
	data.RawHTML = extractRawHTML(e, containerSelector, bodySelector, excludeSelectors)

	// Extract raw text - from HTML or direct extraction
	data.RawText = extractRawText(e, containerSelector, bodySelector, excludeSelectors, data.RawHTML)

	// Extract metadata
	extractMetadata(data, e)

	// Generate ID from URL
	data.ID = generateID(sourceURL)

	return data
}

// extractTitle extracts the page title using multiple strategies
func extractTitle(e *colly.HTMLElement, selector string) string {
	// Try selector if provided
	if selector != "" {
		title := extractText(e, selector)
		if title != "" {
			return title
		}
	}

	// Try JSON-LD headline (often cleanest title on news sites)
	jsonldTitle := extractJSONLDHeadline(e)
	if jsonldTitle != "" {
		return jsonldTitle
	}

	// Try OG title
	ogTitle := extractMeta(e, "og:title")
	if ogTitle != "" {
		return ogTitle
	}

	// Try title tag
	title := e.ChildText("title")
	if title != "" {
		return strings.TrimSpace(title)
	}

	// Try h1 as fallback
	h1 := e.ChildText("h1")
	if h1 != "" {
		return strings.TrimSpace(h1)
	}

	return ""
}

// extractJSONLDHeadline extracts the headline/name from JSON-LD schema.
// Supports NewsArticle/Article (headline), BlogPosting/PressRelease (headline),
// Event/SpecialAnnouncement/Report (name).
func extractJSONLDHeadline(e *colly.HTMLElement) string {
	var headline string

	e.DOM.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		if headline != "" {
			return
		}

		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
			return
		}

		typeVal, _ := data["@type"].(string)
		switch typeVal {
		case jsonldTypeNewsArticle, jsonldTypeArticle, jsonldTypeBlogPosting, jsonldTypePressRelease:
			if h, ok := data["headline"].(string); ok && h != "" {
				headline = strings.TrimSpace(h)
			}
		case jsonldTypeEvent, jsonldTypeSpecialAnnouncement, jsonldTypeReport:
			if n, ok := data["name"].(string); ok && n != "" {
				headline = strings.TrimSpace(n)
			}
		}
	})

	return headline
}

const (
	minHTMLContentLength = 50
	minParagraphLength   = 20
	defaultSchemaOrgURL  = "https://schema.org"
)

// extractRawHTML extracts the raw HTML content from the page
func extractRawHTML(e *colly.HTMLElement, containerSelector, bodySelector string, excludeSelectors []string) string {
	// Try container selector first
	if html := tryExtractHTMLFromSelector(e, containerSelector, excludeSelectors); html != "" {
		return html
	}

	// Try body selector
	if html := tryExtractHTMLFromSelector(e, bodySelector, excludeSelectors); html != "" {
		return html
	}

	// Fallback: try common content containers
	fallbackSelectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		"[role='main']",
		"[role='article']",
	}

	for _, sel := range fallbackSelectors {
		if html := tryExtractHTMLFromSelector(e, sel, excludeSelectors); html != "" {
			if len(strings.TrimSpace(html)) > minHTMLContentLength {
				return html
			}
		}
	}

	// Last resort: get body HTML (excluding common non-content areas)
	return extractBodyHTML(e)
}

// tryExtractHTMLFromSelector attempts to extract HTML from a selector
func tryExtractHTMLFromSelector(e *colly.HTMLElement, selector string, excludeSelectors []string) string {
	if selector == "" {
		return ""
	}

	container := e.DOM.Find(selector).First()
	if container.Length() == 0 {
		return ""
	}

	// Apply excludes
	applyExcludes(container, excludeSelectors)

	html, _ := container.Html()
	return html
}

// applyExcludes applies exclude selectors to a container
func applyExcludes(container *goquery.Selection, excludeSelectors []string) {
	for _, exclude := range excludeSelectors {
		if exclude != "" {
			container.Find(exclude).Remove()
		}
	}
}

// extractBodyHTML extracts HTML from body element, removing non-content areas
func extractBodyHTML(e *colly.HTMLElement) string {
	body := e.DOM.Find("body")
	if body.Length() == 0 {
		return ""
	}

	// Remove common non-content elements
	body.Find("header, footer, nav, aside, .header, .footer, .navigation, .sidebar, .menu, script, style").Remove()
	html, _ := body.Html()
	return html
}

// extractRawText extracts plain text from the page
func extractRawText(
	e *colly.HTMLElement,
	containerSelector, bodySelector string,
	excludeSelectors []string,
	rawHTML string,
) string {
	// If we have raw HTML, extract text from it
	if rawHTML != "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err == nil {
			text := doc.Text()
			cleaned := strings.TrimSpace(text)
			if cleaned != "" {
				return cleaned
			}
		}
	}

	// Try container selector
	if containerSelector != "" {
		text := extractTextFromContainer(e, containerSelector, excludeSelectors)
		if text != "" {
			return text
		}
	}

	// Try body selector
	if bodySelector != "" {
		text := extractText(e, bodySelector)
		if text != "" {
			return text
		}
	}

	// Fallback: try common content containers
	fallbackSelectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		"[role='main']",
		"[role='article']",
	}

	for _, sel := range fallbackSelectors {
		text := extractTextFromContainer(e, sel, excludeSelectors)
		if text != "" && len(strings.TrimSpace(text)) > 50 {
			return text
		}
	}

	// Last resort: extract from body paragraphs
	return extractFromBodyParagraphs(e, excludeSelectors)
}

// extractFromBodyParagraphs extracts text from body paragraphs
func extractFromBodyParagraphs(e *colly.HTMLElement, excludeSelectors []string) string {
	body := e.DOM.Find("body")
	if body.Length() == 0 {
		return ""
	}

	// Remove common non-content elements
	body.Find("header, footer, nav, aside, .header, .footer, .navigation, .sidebar, .menu, script, style").Remove()

	// Apply excludes
	for _, exclude := range excludeSelectors {
		if exclude != "" {
			body.Find(exclude).Remove()
		}
	}

	// Get all paragraphs
	paragraphs := body.Find("p")
	if paragraphs.Length() == 0 {
		// If no paragraphs, just get all text
		return strings.TrimSpace(body.Text())
	}

	var textParts []string
	paragraphs.Each(func(i int, s *goquery.Selection) {
		paraText := strings.TrimSpace(s.Text())
		if len(paraText) > minParagraphLength {
			textParts = append(textParts, paraText)
		}
	})

	if len(textParts) == 0 {
		return strings.TrimSpace(body.Text())
	}

	return strings.Join(textParts, "\n\n")
}

// extractJSONLD extracts JSON-LD structured data from script tags
func extractJSONLD(e *colly.HTMLElement) map[string]any {
	result := make(map[string]any)

	// Find all script tags with type="application/ld+json"
	e.DOM.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return
		}

		var jsonData any
		if err := json.Unmarshal([]byte(jsonText), &jsonData); err != nil {
			// Skip invalid JSON, continue processing other scripts
			return
		}

		// Handle both single objects and arrays
		var jsonObjs []any
		switch v := jsonData.(type) {
		case []any:
			jsonObjs = v
		case map[string]any:
			jsonObjs = []any{v}
		default:
			return
		}

		// Extract schema data for supported content types
		for _, obj := range jsonObjs {
			objMap, isMap := obj.(map[string]any)
			if !isMap {
				continue
			}

			typeVal, _ := objMap["@type"].(string)
			if typeVal == "" {
				continue
			}

			// Dispatch to schema-specific extractor; skip unsupported types
			var extracted bool
			switch typeVal {
			case jsonldTypeNewsArticle, jsonldTypeArticle:
				extractNewsArticleFields(objMap, result)
				result["jsonld_schema_type"] = typeVal
				extracted = true
			case jsonldTypeBlogPosting:
				extractNewsArticleFields(objMap, result)
				result["jsonld_schema_type"] = jsonldTypeBlogPosting
				extracted = true
			case jsonldTypePressRelease:
				extractNewsArticleFields(objMap, result)
				result["jsonld_schema_type"] = jsonldTypePressRelease
				extracted = true
			case jsonldTypeEvent:
				extractEventFields(objMap, result)
				result["jsonld_schema_type"] = jsonldTypeEvent
				extracted = true
			case jsonldTypeSpecialAnnouncement:
				extractSpecialAnnouncementFields(objMap, result)
				result["jsonld_schema_type"] = jsonldTypeSpecialAnnouncement
				extracted = true
			case jsonldTypeReport:
				extractReportFields(objMap, result)
				result["jsonld_schema_type"] = jsonldTypeReport
				extracted = true
			default:
				// Unsupported schema type, skip
				continue
			}

			if extracted {
				result["jsonld_raw"] = normalizeJSONLDObject(objMap)
			}
		}
	})

	return result
}

// extractNewsArticleFields extracts fields from a NewsArticle JSON-LD object
func extractNewsArticleFields(objMap, result map[string]any) {
	extractJSONLDStringFields(objMap, result)
	extractJSONLDNumericFields(objMap, result)
	extractJSONLDKeywords(objMap, result)
	extractJSONLDAuthor(objMap, result)
	extractJSONLDPublisher(objMap, result)
	extractJSONLDImage(objMap, result)
}

// extractEventFields extracts fields from an Event JSON-LD object.
// Maps name->headline, startDate->datePublished, description->description.
func extractEventFields(objMap, result map[string]any) {
	extractJSONLDEventStringFields(objMap, result)
	extractJSONLDAuthor(objMap, result)
	extractJSONLDImage(objMap, result)
}

// extractJSONLDEventStringFields maps Event schema fields to our standard keys.
func extractJSONLDEventStringFields(objMap, result map[string]any) {
	fieldMap := map[string]string{
		"name":        "jsonld_headline",
		"description": "jsonld_description",
		"url":         "jsonld_url",
	}
	for key, resultKey := range fieldMap {
		if val, isString := objMap[key].(string); isString && val != "" {
			result[resultKey] = val
		}
	}
	if startDate, ok := objMap["startDate"].(string); ok && startDate != "" {
		result["jsonld_date_published"] = startDate
	}
	if loc, locOk := objMap["location"].(map[string]any); locOk {
		if locName, nameOk := loc["name"].(string); nameOk && locName != "" {
			result["jsonld_location"] = locName
		}
	}
}

// extractSpecialAnnouncementFields extracts fields from a SpecialAnnouncement JSON-LD object.
// Maps name->headline, datePosted->datePublished, text->description.
func extractSpecialAnnouncementFields(objMap, result map[string]any) {
	fieldMap := map[string]string{
		"name": "jsonld_headline",
		"text": "jsonld_description",
	}
	for key, resultKey := range fieldMap {
		if val, isString := objMap[key].(string); isString && val != "" {
			result[resultKey] = val
		}
	}
	if datePosted, ok := objMap["datePosted"].(string); ok && datePosted != "" {
		result["jsonld_date_published"] = datePosted
	}
	extractJSONLDAuthor(objMap, result)
}

// extractReportFields extracts fields from a Report JSON-LD object.
func extractReportFields(objMap, result map[string]any) {
	fieldMap := map[string]string{
		"name":          "jsonld_headline",
		"description":   "jsonld_description",
		"url":           "jsonld_url",
		"datePublished": "jsonld_date_published",
	}
	for key, resultKey := range fieldMap {
		if val, isString := objMap[key].(string); isString && val != "" {
			result[resultKey] = val
		}
	}
	extractJSONLDAuthor(objMap, result)
}

// extractJSONLDStringFields extracts string fields from JSON-LD object
func extractJSONLDStringFields(objMap, result map[string]any) {
	fieldMap := map[string]string{
		"headline":       "jsonld_headline",
		"description":    "jsonld_description",
		"articleSection": "jsonld_article_section",
		"url":            "jsonld_url",
		"dateCreated":    "jsonld_date_created",
		"dateModified":   "jsonld_date_modified",
		"datePublished":  "jsonld_date_published",
	}

	for key, resultKey := range fieldMap {
		if val, isString := objMap[key].(string); isString && val != "" {
			result[resultKey] = val
		}
	}
}

// extractJSONLDNumericFields extracts numeric fields from JSON-LD object
func extractJSONLDNumericFields(objMap, result map[string]any) {
	if wordCount, isFloat := objMap["wordCount"].(float64); isFloat {
		result["jsonld_word_count"] = int(wordCount)
	}
}

// extractJSONLDKeywords extracts keywords array from JSON-LD object
func extractJSONLDKeywords(objMap, result map[string]any) {
	keywords, isArray := objMap["keywords"].([]any)
	if !isArray || len(keywords) == 0 {
		return
	}

	keywordStrs := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if kwStr, isKwString := kw.(string); isKwString {
			keywordStrs = append(keywordStrs, kwStr)
		}
	}
	if len(keywordStrs) > 0 {
		result["jsonld_keywords"] = keywordStrs
	}
}

// extractJSONLDAuthor extracts author field from JSON-LD object (can be string or object)
func extractJSONLDAuthor(objMap, result map[string]any) {
	if author, isAuthorString := objMap["author"].(string); isAuthorString && author != "" {
		result["jsonld_author"] = author
		return
	}

	if authorObj, isAuthorObj := objMap["author"].(map[string]any); isAuthorObj {
		if authorName, isNameString := authorObj["name"].(string); isNameString && authorName != "" {
			result["jsonld_author"] = authorName
		}
	}
}

// extractJSONLDPublisher extracts publisher field from JSON-LD object
func extractJSONLDPublisher(objMap, result map[string]any) {
	publisher, isPublisherObj := objMap["publisher"].(map[string]any)
	if !isPublisherObj {
		return
	}

	if pubName, isPubNameString := publisher["name"].(string); isPubNameString && pubName != "" {
		result["jsonld_publisher_name"] = pubName
	}
}

// extractJSONLDImage extracts image field from JSON-LD object (can be object or string)
func extractJSONLDImage(objMap, result map[string]any) {
	if image, isImageObj := objMap["image"].(map[string]any); isImageObj {
		if imageURL, isImageURLString := image["url"].(string); isImageURLString && imageURL != "" {
			result["jsonld_image_url"] = imageURL
		}
		return
	}

	if imageStr, isImageString := objMap["image"].(string); isImageString && imageStr != "" {
		result["jsonld_image_url"] = imageStr
	}
}

// normalizeJSONLDObject normalizes a JSON-LD object to prevent Elasticsearch mapping conflicts.
// Fields like @context and author can be string, object, or array depending on the page,
// which causes ES dynamic mapping conflicts. Normalize them to consistent types.
func normalizeJSONLDObject(objMap map[string]any) map[string]any {
	// Create a deep copy to avoid mutating the original
	normalized := make(map[string]any, len(objMap))

	// Copy all fields
	for key, val := range objMap {
		normalized[key] = val
	}

	// Normalize @context to always be a string (prevents ES mapping conflicts
	// when some pages have "@context": "https://schema.org" and others have
	// "@context": {"@vocab": "https://schema.org/"})
	if ctxVal, hasCtx := normalized["@context"]; hasCtx {
		normalized["@context"] = normalizeContextField(ctxVal)
	}

	// Normalize author field to always be a string
	if authorVal, hasAuthor := normalized["author"]; hasAuthor {
		normalized["author"] = normalizeAuthorField(authorVal)
		if normalized["author"] == nil {
			delete(normalized, "author")
		}
	}

	// Normalize image to always be a string URL
	if imgVal, hasImg := normalized["image"]; hasImg {
		normalized["image"] = normalizeImageField(imgVal)
		if normalized["image"] == nil {
			delete(normalized, "image")
		}
	}

	// Normalize publisher to always be a string name
	if pubVal, hasPub := normalized["publisher"]; hasPub {
		normalized["publisher"] = normalizeObjectToName(pubVal)
		if normalized["publisher"] == nil {
			delete(normalized, "publisher")
		}
	}

	// Normalize mainEntityOfPage to always be a string URL
	if meVal, hasME := normalized["mainEntityOfPage"]; hasME {
		normalized["mainEntityOfPage"] = normalizeEntityToURL(meVal)
		if normalized["mainEntityOfPage"] == nil {
			delete(normalized, "mainEntityOfPage")
		}
	}

	// Normalize wordCount to always be a string (some sites emit it as int, others as string)
	if wcVal, hasWC := normalized["wordCount"]; hasWC {
		normalized["wordCount"] = normalizeToString(wcVal)
	}

	return normalized
}

// normalizeContextField normalizes JSON-LD @context to always be a string.
// @context can be a string ("https://schema.org"), an object ({"@vocab": "..."}),
// or an array (["https://schema.org", {...}]).
func normalizeContextField(ctxVal any) string {
	switch v := ctxVal.(type) {
	case string:
		return v
	case map[string]any:
		// Extract @vocab or first string value from the context object
		if vocab, ok := v["@vocab"].(string); ok {
			return vocab
		}
		return defaultSchemaOrgURL
	case []any:
		// Extract the first string element from the array
		for _, item := range v {
			if s, ok := item.(string); ok {
				return s
			}
		}
		return defaultSchemaOrgURL
	default:
		return defaultSchemaOrgURL
	}
}

// normalizeImageField normalizes the image field to a string URL.
// Image can be a string URL, an object with "url" field, or an array.
func normalizeImageField(imgVal any) any {
	switch v := imgVal.(type) {
	case string:
		return v
	case map[string]any:
		if u, ok := v["url"].(string); ok && u != "" {
			return u
		}
		return nil
	case []any:
		// Extract URL from first element
		for _, item := range v {
			switch img := item.(type) {
			case string:
				return img
			case map[string]any:
				if u, ok := img["url"].(string); ok && u != "" {
					return u
				}
			}
		}
		return nil
	default:
		return nil
	}
}

// normalizeObjectToName extracts a string name from a field that can be string or object.
// Used for publisher and similar fields that have a "name" property.
func normalizeObjectToName(val any) any {
	switch v := val.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok && name != "" {
			return name
		}
		return nil
	default:
		return nil
	}
}

// NormalizeJSONLDRawForIndex ensures jsonld_raw inside jsonLDData never contains
// object or array values for polymorphic fields (publisher, author, image,
// mainEntityOfPage), so indexing never sends types that conflict with
// Elasticsearch mapping. Mutates jsonLDData in place when jsonld_raw is present.
func NormalizeJSONLDRawForIndex(jsonLDData map[string]any) {
	if jsonLDData == nil {
		return
	}
	raw, ok := jsonLDData["jsonld_raw"].(map[string]any)
	if !ok {
		return
	}
	// Publisher: string or object (or array — take first) → string or remove
	if pubVal, hasPub := raw["publisher"]; hasPub {
		normalized := normalizePublisherValue(pubVal)
		if normalized == nil {
			delete(raw, "publisher")
		} else {
			raw["publisher"] = normalized
		}
	}
	// Author: string/object/array → string or remove
	if authorVal, hasAuthor := raw["author"]; hasAuthor {
		normalized := normalizeAuthorField(authorVal)
		if normalized == nil {
			delete(raw, "author")
		} else {
			raw["author"] = normalized
		}
	}
	// Image: string/object/array → string URL or remove
	if imgVal, hasImg := raw["image"]; hasImg {
		normalized := normalizeImageField(imgVal)
		if normalized == nil {
			delete(raw, "image")
		} else {
			raw["image"] = normalized
		}
	}
	// mainEntityOfPage: string or object (or array — take first) → string URL or remove
	if meVal, hasME := raw["mainEntityOfPage"]; hasME {
		normalized := normalizeMainEntityValue(meVal)
		if normalized == nil {
			delete(raw, "mainEntityOfPage")
		} else {
			raw["mainEntityOfPage"] = normalized
		}
	}
}

// normalizePublisherValue normalizes publisher (string, object, or array) to a string or nil.
func normalizePublisherValue(val any) any {
	if arr, ok := val.([]any); ok && len(arr) > 0 {
		return normalizeObjectToName(arr[0])
	}
	return normalizeObjectToName(val)
}

// normalizeMainEntityValue normalizes mainEntityOfPage (string, object, or array) to a string URL or nil.
func normalizeMainEntityValue(val any) any {
	if arr, ok := val.([]any); ok && len(arr) > 0 {
		return normalizeEntityToURL(arr[0])
	}
	return normalizeEntityToURL(val)
}

// normalizeEntityToURL extracts a string URL from mainEntityOfPage.
// Can be a string URL or an object with "@id" or "url" field.
func normalizeEntityToURL(val any) any {
	switch v := val.(type) {
	case string:
		return v
	case map[string]any:
		if id, ok := v["@id"].(string); ok && id != "" {
			return id
		}
		if u, ok := v["url"].(string); ok && u != "" {
			return u
		}
		return nil
	default:
		return nil
	}
}

// normalizeToString converts any scalar value to its string representation.
func normalizeToString(val any) string {
	return fmt.Sprintf("%v", val)
}

// normalizeAuthorField normalizes the author field from various types to a string.
func normalizeAuthorField(authorVal any) any {
	switch v := authorVal.(type) {
	case string:
		// Already a string, keep it
		return v
	case map[string]any:
		// Convert object to string (extract name if available)
		return extractAuthorNameFromObject(v)
	case []any:
		// Handle array of authors - extract names and join them
		return extractAuthorNamesFromArray(v)
	default:
		// Unknown type, return nil to signal removal
		return nil
	}
}

// extractAuthorNameFromObject extracts the author name from an object.
func extractAuthorNameFromObject(authorObj map[string]any) any {
	if name, hasName := authorObj["name"].(string); hasName && name != "" {
		return name
	}
	// If no name field, return nil to signal removal
	return nil
}

// extractAuthorNamesFromArray extracts author names from an array and joins them.
func extractAuthorNamesFromArray(authorArr []any) any {
	authorNames := make([]string, 0, len(authorArr))
	for _, item := range authorArr {
		switch itemVal := item.(type) {
		case string:
			authorNames = append(authorNames, itemVal)
		case map[string]any:
			if name, hasName := itemVal["name"].(string); hasName && name != "" {
				authorNames = append(authorNames, name)
			}
		}
	}
	if len(authorNames) > 0 {
		// Join multiple authors with comma
		return strings.Join(authorNames, ", ")
	}
	// If no valid authors found, return nil to signal removal
	return nil
}

// extractArticleMeta extracts article-specific metadata
func extractArticleMeta(e *colly.HTMLElement) ArticleMeta {
	var meta ArticleMeta

	meta.Section = extractMeta(e, "article:section")
	meta.ContentTier = extractMeta(e, "article:content_tier")

	// Parse article:opinion as boolean
	opinionStr := extractMeta(e, "article:opinion")
	meta.Opinion = opinionStr == "true" || opinionStr == "1"

	return meta
}

// extractTwitterMeta extracts Twitter card metadata
func extractTwitterMeta(e *colly.HTMLElement) TwitterMeta {
	var meta TwitterMeta

	meta.Card = extractMeta(e, "twitter:card")
	meta.Site = extractMeta(e, "twitter:site")

	return meta
}

// extractExtendedOG extracts extended Open Graph metadata
func extractExtendedOG(e *colly.HTMLElement) ExtendedOG {
	var og ExtendedOG

	og.SiteName = extractMeta(e, "og:site_name")

	// Parse numeric values
	if widthStr := extractMeta(e, "og:image:width"); widthStr != "" {
		if width, err := strconv.Atoi(widthStr); err == nil {
			og.ImageWidth = width
		}
	}
	if heightStr := extractMeta(e, "og:image:height"); heightStr != "" {
		if height, err := strconv.Atoi(heightStr); err == nil {
			og.ImageHeight = height
		}
	}

	return og
}

// extractMetadata extracts Open Graph and other metadata
func extractMetadata(data *RawContentData, e *colly.HTMLElement) {
	// Extract basic meta tags (keep existing extraction for backward compatibility)
	data.MetaDescription = extractMeta(e, "description")
	data.MetaKeywords = extractMeta(e, "keywords")
	data.OGType = extractMeta(e, "og:type")
	data.OGTitle = extractMeta(e, "og:title")
	data.OGDescription = extractMeta(e, "og:description")
	data.OGImage = extractMeta(e, "og:image")
	data.OGURL = extractMeta(e, "og:url")
	data.CanonicalURL = extractAttr(e, "link[rel='canonical']", "href")
	data.Author = extractMeta(e, "author")

	// Try to extract published date from meta tags
	if dateStr := extractMeta(e, "article:published_time"); dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			data.PublishedDate = &t
		}
	}
	if data.PublishedDate == nil {
		if dateStr := extractMeta(e, "article:published"); dateStr != "" {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				data.PublishedDate = &t
			}
		}
	}

	// Use specialized extractors (SRP)
	jsonLDData := extractJSONLD(e)
	if len(jsonLDData) > 0 {
		data.JSONLDData = jsonLDData
	} else {
		data.JSONLDData = make(map[string]any)
	}

	// Extended date fallbacks (require JSON-LD data)
	extractDateFallbacks(data, e)

	// Extended author fallbacks (require JSON-LD data)
	extractAuthorFallbacks(data, e)

	articleMeta := extractArticleMeta(e)
	data.ArticleSection = articleMeta.Section
	data.ArticleOpinion = articleMeta.Opinion
	data.ArticleContentTier = articleMeta.ContentTier

	twitterMeta := extractTwitterMeta(e)
	data.TwitterCard = twitterMeta.Card
	data.TwitterSite = twitterMeta.Site

	extendedOG := extractExtendedOG(e)
	data.OGImageWidth = extendedOG.ImageWidth
	data.OGImageHeight = extendedOG.ImageHeight
	data.OGSiteName = extendedOG.SiteName
}

// dateCSSSelectors are common CSS class selectors for published date elements.
var dateCSSSelectors = []string{".published-date", ".post-date", ".entry-date", ".article-date"}

// extractDateFallbacks applies additional date extraction strategies after JSON-LD data is available.
func extractDateFallbacks(data *RawContentData, e *colly.HTMLElement) {
	// Try JSON-LD datePublished
	if data.PublishedDate == nil && len(data.JSONLDData) > 0 {
		if dateStr, ok := data.JSONLDData["jsonld_date_published"].(string); ok && dateStr != "" {
			if t, parseErr := time.Parse(time.RFC3339, dateStr); parseErr == nil {
				data.PublishedDate = &t
			}
		}
	}

	// Try <time datetime="..."> element
	if data.PublishedDate == nil {
		if dateStr := e.ChildAttr("time[datetime]", "datetime"); dateStr != "" {
			if t, parseErr := time.Parse(time.RFC3339, dateStr); parseErr == nil {
				data.PublishedDate = &t
			}
		}
	}

	// Try common CSS class selectors for published date
	if data.PublishedDate == nil {
		extractDateFromCSSSelectors(data, e)
	}
}

// extractDateFromCSSSelectors tries common CSS class selectors for published date.
func extractDateFromCSSSelectors(data *RawContentData, e *colly.HTMLElement) {
	for _, sel := range dateCSSSelectors {
		dateText := e.ChildAttr(sel+" time", "datetime")
		if dateText == "" {
			dateText = e.ChildText(sel)
		}

		if dateText != "" {
			if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(dateText)); parseErr == nil {
				data.PublishedDate = &t
				return
			}
		}
	}
}

// bylineCSSSelectors are common CSS class selectors for author/byline elements.
var bylineCSSSelectors = []string{".byline", ".author", ".post-author", ".article-author"}

// extractAuthorFallbacks applies additional author extraction strategies after JSON-LD data is available.
func extractAuthorFallbacks(data *RawContentData, e *colly.HTMLElement) {
	// Try JSON-LD author
	if data.Author == "" && len(data.JSONLDData) > 0 {
		if author, ok := data.JSONLDData["jsonld_author"].(string); ok && author != "" {
			data.Author = author
		}
	}

	// Try rel="author" link
	if data.Author == "" {
		data.Author = strings.TrimSpace(e.ChildText("a[rel='author']"))
	}

	// Try common byline selectors
	if data.Author == "" {
		extractAuthorFromBylineSelectors(data, e)
	}
}

// extractAuthorFromBylineSelectors tries common CSS byline selectors for author.
func extractAuthorFromBylineSelectors(data *RawContentData, e *colly.HTMLElement) {
	for _, sel := range bylineCSSSelectors {
		author := strings.TrimSpace(e.ChildText(sel))
		if author != "" {
			data.Author = author
			return
		}
	}
}

// extractText extracts text from the first element matching the selector
func extractText(e *colly.HTMLElement, selector string) string {
	if selector == "" {
		return ""
	}
	selectors := strings.Split(selector, ",")
	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}
		childText := e.ChildText(sel)
		if childText != "" {
			return strings.TrimSpace(childText)
		}
		element := e.DOM.Find(sel).First()
		if element.Length() > 0 {
			elementText := element.Text()
			if elementText != "" {
				return strings.TrimSpace(elementText)
			}
		}
	}
	return ""
}

// extractTextFromContainer extracts text from a container element
func extractTextFromContainer(e *colly.HTMLElement, containerSelector string, excludeSelectors []string) string {
	if containerSelector == "" {
		return ""
	}

	selectors := strings.Split(containerSelector, ",")
	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}

		container := e.DOM.Find(sel).First()
		if container.Length() == 0 {
			continue
		}

		// Apply excludes
		for _, exclude := range excludeSelectors {
			if exclude != "" {
				container.Find(exclude).Remove()
			}
		}

		text := container.Text()
		if text != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// extractMeta extracts a meta tag value
func extractMeta(e *colly.HTMLElement, property string) string {
	// Try property attribute first (for og: tags)
	selector := fmt.Sprintf("meta[property='%s']", property)
	value := e.ChildAttr(selector, "content")
	if value != "" {
		return value
	}

	// Try name attribute (for standard meta tags)
	selector = fmt.Sprintf("meta[name='%s']", property)
	return e.ChildAttr(selector, "content")
}

// extractAttr extracts an attribute from an element
func extractAttr(e *colly.HTMLElement, selector, attr string) string {
	return e.ChildAttr(selector, attr)
}

// generateID generates a unique ID from the URL
func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

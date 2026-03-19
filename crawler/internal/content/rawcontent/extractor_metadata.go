package rawcontent

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

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

package rawcontent

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

const (
	minHTMLContentLength = 50
	minParagraphLength   = 20
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

	// Text density heuristic: score elements by (non-link content)² / total text.
	if html := extractHTMLByTextDensity(e); html != "" {
		if len(strings.TrimSpace(html)) >= minHTMLContentLength {
			return html
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

	// Text density heuristic: score elements by (non-link content)² / total text.
	if densityText := extractByTextDensity(e); densityText != "" {
		if len(strings.TrimSpace(densityText)) >= minHTMLContentLength {
			return densityText
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

// Text Density Heuristic
// ----------------------
// Identifies the most content-rich element in the DOM by scoring candidate
// elements by (non-link content length)² / total text length. This rewards
// elements that are both voluminous and not dominated by navigation links.

const textDensityMinChars = 200

// densityNoiseFragments are substrings matched against an element's class and id
// attributes. Elements matching any fragment are excluded from density scoring.
var densityNoiseFragments = []string{
	"nav", "menu", "sidebar", "header", "footer", "ad-", "banner",
	"promo", "comment", "social", "related", "widget",
}

// isDensityNoiseElement returns true when the element's class or id contains a
// known noise fragment (navigation, ads, sidebar, etc.).
// Matches are word-boundary-aware: class/id values are split on whitespace and
// hyphens, then each token is checked for a prefix match against noise fragments.
// This prevents "nav" from matching "navigate" while still matching "nav-bar".
func isDensityNoiseElement(s *goquery.Selection) bool {
	class, _ := s.Attr("class")
	id, _ := s.Attr("id")
	combined := strings.ToLower(class + " " + id)

	tokens := splitClassTokens(combined)
	for _, fragment := range densityNoiseFragments {
		for _, token := range tokens {
			if token == fragment || strings.HasPrefix(token, fragment) {
				return true
			}
		}
	}
	return false
}

// splitClassTokens splits a combined class+id string into individual tokens by
// whitespace, hyphens, and underscores — the common CSS naming delimiters.
func splitClassTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})
}

// ancestorScoreThreshold is the fraction of a parent's score that a child must
// reach to be preferred over its ancestor. A child scoring >= 80% of the parent
// is considered a better (more specific) pick.
const ancestorScoreThreshold = 0.8

// findDensestElement walks div/section/article/main descendants of <body> and
// returns the element with the highest density score. Score is defined as
// contentLen² / totalLen, where contentLen = totalLen − linkTextLen. This
// rewards elements that are voluminous and not dominated by navigation links.
// When a child element scores at least 80% of an ancestor's score, the child
// is preferred to avoid selecting overly broad containers.
// Returns nil if no element meets the textDensityMinChars threshold.
func findDensestElement(e *colly.HTMLElement) *goquery.Selection {
	body := e.DOM.Find("body")
	if body.Length() == 0 {
		return nil
	}

	var best *goquery.Selection
	var bestScore float64

	body.Find("div, section, article, main").Each(func(_ int, s *goquery.Selection) {
		if isDensityNoiseElement(s) {
			return
		}

		totalText := strings.TrimSpace(s.Text())
		totalLen := len(totalText)
		if totalLen < textDensityMinChars {
			return
		}

		// Measure how much of the text lives inside links (navigation signal).
		linkLen := 0
		s.Find("a").Each(func(_ int, a *goquery.Selection) {
			linkLen += len(strings.TrimSpace(a.Text()))
		})

		contentLen := totalLen - linkLen
		if contentLen <= 0 {
			return
		}

		// score = contentLen² / totalLen — rewards density AND volume.
		score := float64(contentLen) * float64(contentLen) / float64(totalLen)

		if best == nil {
			bestScore = score
			best = s
			return
		}

		if score > bestScore {
			bestScore = score
			best = s
		} else if score >= bestScore*ancestorScoreThreshold && isDescendantOf(s, best) {
			// Child scores nearly as high as ancestor — prefer the more specific element.
			bestScore = score
			best = s
		}
	})

	return best
}

// isDescendantOf returns true when child is a DOM descendant of ancestor.
// Walks up from child via Parents(), which is O(depth) instead of O(subtree).
func isDescendantOf(child, ancestor *goquery.Selection) bool {
	ancestorNode := ancestor.Get(0)
	found := false
	child.Parents().Each(func(_ int, parent *goquery.Selection) {
		if parent.Get(0) == ancestorNode {
			found = true
		}
	})
	return found
}

// extractByTextDensity returns the plain text of the densest content element,
// or "" when no qualifying element is found.
func extractByTextDensity(e *colly.HTMLElement) string {
	best := findDensestElement(e)
	if best == nil {
		return ""
	}
	return strings.TrimSpace(best.Text())
}

// extractHTMLByTextDensity returns the inner HTML of the densest content element,
// or "" when no qualifying element is found or HTML serialization fails.
func extractHTMLByTextDensity(e *colly.HTMLElement) string {
	best := findDensestElement(e)
	if best == nil {
		return ""
	}
	html, err := best.Html()
	if err != nil {
		return ""
	}
	return html
}

// Package rawcontent provides functionality for extracting raw content from any HTML page.
package rawcontent

import (
	"net/url"
	"strings"

	readability "github.com/go-shiori/go-readability"
)

// ApplyReadabilityFallback runs a readability-style extractor on the full document HTML
// and returns updated title, rawHTML, and rawText. Use only when selector-based extraction
// yielded empty or negligible content. Returns empty strings if readability fails or yields nothing.
func ApplyReadabilityFallback(documentHTML, pageURL string) (title, rawHTML, rawText string) {
	documentHTML = strings.TrimSpace(documentHTML)
	if documentHTML == "" {
		return "", "", ""
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return "", "", ""
	}

	article, err := readability.FromReader(strings.NewReader(documentHTML), parsedURL)
	if err != nil {
		return "", "", ""
	}

	title = strings.TrimSpace(article.Title)
	rawHTML = strings.TrimSpace(article.Content)
	rawText = strings.TrimSpace(article.TextContent)

	return title, rawHTML, rawText
}

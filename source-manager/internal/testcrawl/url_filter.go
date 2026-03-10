// Package testcrawl provides a lightweight extraction pipeline for the test-crawl endpoint.
// It replicates the crawler's content extraction logic without importing the crawler module.
package testcrawl

import (
	"strings"
)

// binaryExtensions lists file extensions that are never crawlable HTML content.
var binaryExtensions = []string{
	".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	".zip", ".tar", ".gz", ".rar", ".7z",
	".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico",
	".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv",
	".css", ".js", ".woff", ".woff2", ".ttf", ".eot",
	".xml", ".json", ".csv",
}

// cdnHostFragments lists host substrings that indicate CDN/static asset domains.
var cdnHostFragments = []string{
	"cdn.", "assets.", "static.", "media.", "images.", "img.",
	"s3.amazonaws.com", "storage.googleapis.com", "cloudfront.net",
}

// URLFilterResult holds the result of the URL pre-filter check.
type URLFilterResult struct {
	Allowed bool
	Reason  string
}

// FilterURL checks whether a URL should be fetched for content extraction.
// Returns allowed=false with a reason string when the URL should be skipped.
func FilterURL(rawURL string) URLFilterResult {
	lower := strings.ToLower(rawURL)

	// Strip query string for extension matching.
	path := lower
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	for _, ext := range binaryExtensions {
		if strings.HasSuffix(path, ext) {
			return URLFilterResult{Allowed: false, Reason: "binary/non-HTML extension: " + ext}
		}
	}

	for _, fragment := range cdnHostFragments {
		if strings.Contains(lower, fragment) {
			return URLFilterResult{Allowed: false, Reason: "CDN/asset host: " + fragment}
		}
	}

	return URLFilterResult{Allowed: true}
}

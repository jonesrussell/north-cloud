package crawler

import (
	"net/url"
	"path"
	"strings"
)

// ecommerceSegments are URL path segments indicating e-commerce pages (non-content).
var ecommerceSegments = map[string]bool{
	"shop":     true,
	"store":    true,
	"product":  true,
	"products": true,
	"cart":     true,
	"checkout": true,
}

// cdnAssetPrefixes are URL path prefixes for CDN/asset directories.
var cdnAssetPrefixes = []string{"/wp-content/uploads/", "/assets/", "/static/"}

// shouldSkipURL returns true if the URL should be skipped (non-content resource).
// It checks for binary file extensions, CDN/asset paths, non-content segments,
// and e-commerce segments.
func shouldSkipURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	lowerPath := strings.ToLower(parsed.Path)

	if hasBinaryExtension(lowerPath) {
		return true
	}

	if isCDNAssetPath(lowerPath) {
		return true
	}

	return hasNonContentOrEcommerceSegment(lowerPath)
}

// hasBinaryExtension checks if the path ends with a known binary file extension.
func hasBinaryExtension(lowerPath string) bool {
	ext := path.Ext(lowerPath)
	return ext != "" && binaryExtensions[ext]
}

// isCDNAssetPath checks if the path starts with a known CDN/asset prefix
// and also has a binary file extension.
func isCDNAssetPath(lowerPath string) bool {
	for _, prefix := range cdnAssetPrefixes {
		if strings.HasPrefix(lowerPath, prefix) && hasBinaryExtension(lowerPath) {
			return true
		}
	}

	return false
}

// hasNonContentOrEcommerceSegment checks if any path segment is a known
// non-content or e-commerce segment.
func hasNonContentOrEcommerceSegment(lowerPath string) bool {
	segments := strings.Split(strings.TrimLeft(lowerPath, "/"), "/")

	for _, seg := range segments {
		if nonContentSegments[seg] || ecommerceSegments[seg] {
			return true
		}
	}

	return false
}

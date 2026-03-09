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

// nonContentHosts are exact hostnames or host suffixes that never serve article content.
// Suffix matches use a leading dot (e.g. ".cloudfront.net" matches any subdomain).
var nonContentHosts = []string{
	"play.google.com",
	"apps.apple.com",
	"itunes.apple.com",
	".cloudfront.net",
	".googleusercontent.com",
	".fbcdn.net",
	".twimg.com",
}

// shouldSkipURL returns true if the URL should be skipped (non-content resource).
// It checks for off-domain hosts, non-content hosts, binary file extensions,
// CDN/asset paths, non-content segments, and e-commerce segments.
// sourceHost is the hostname of the crawl source; an empty string disables the off-domain check.
func shouldSkipURL(rawURL, sourceHost string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	lowerHost := strings.ToLower(parsed.Hostname())

	if sourceHost != "" && !strings.EqualFold(lowerHost, sourceHost) {
		return true
	}

	if isNonContentHost(lowerHost) {
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

// isNonContentHost returns true if the hostname is a known non-content domain.
// Exact matches and suffix matches (leading dot) are both supported.
func isNonContentHost(lowerHost string) bool {
	for _, entry := range nonContentHosts {
		if strings.HasPrefix(entry, ".") {
			if strings.HasSuffix(lowerHost, entry) {
				return true
			}
		} else if lowerHost == entry {
			return true
		}
	}
	return false
}

// hasBinaryExtension checks if the path ends with a known binary file extension.
func hasBinaryExtension(lowerPath string) bool {
	ext := path.Ext(lowerPath)
	return ext != "" && binaryExtensions[ext]
}

// isCDNAssetPath checks if the path starts with a known CDN/asset prefix.
// CDN asset directories (/wp-content/uploads/, /assets/, /static/) are skipped
// regardless of file extension — they never contain article content.
func isCDNAssetPath(lowerPath string) bool {
	for _, prefix := range cdnAssetPrefixes {
		if strings.HasPrefix(lowerPath, prefix) {
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

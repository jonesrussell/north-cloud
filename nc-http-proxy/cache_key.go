package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Tracking parameters to strip from URLs.
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"fbclid":       true,
	"gclid":        true,
	"ref":          true,
}

// NormalizeDomain converts a domain to a directory-safe format.
// Lowercase, strip www prefix, replace dots with dashes.
func NormalizeDomain(domain string) string {
	d := strings.ToLower(domain)
	d = strings.TrimPrefix(d, "www.")
	d = strings.ReplaceAll(d, ".", "-")
	return d
}

// NormalizeURL normalizes a URL for cache key generation.
// Lowercase host, sort query params, strip tracking params.
func NormalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsed.Host = strings.ToLower(parsed.Host)

	query := parsed.Query()
	for param := range trackingParams {
		query.Del(param)
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sortedQuery strings.Builder
	for i, k := range keys {
		if i > 0 {
			sortedQuery.WriteByte('&')
		}
		sortedQuery.WriteString(url.QueryEscape(k))
		sortedQuery.WriteByte('=')
		sortedQuery.WriteString(url.QueryEscape(query.Get(k)))
	}

	parsed.RawQuery = sortedQuery.String()
	return parsed.String()
}

const hashPrefixLength = 12

// GenerateCacheKey creates a deterministic cache key for a request.
// Format: METHOD_sha256(normalized_url + "\n" + header_hash)[:12]
func GenerateCacheKey(req *http.Request) string {
	normalizedURL := NormalizeURL(req.URL.String())

	headerHash := hashHeaders(req.Header)

	combined := normalizedURL + "\n" + headerHash
	hash := sha256.Sum256([]byte(combined))
	shortHash := hex.EncodeToString(hash[:])[:hashPrefixLength]

	return req.Method + "_" + shortHash
}

// hashHeaders creates a deterministic hash of relevant headers.
func hashHeaders(headers http.Header) string {
	userAgent := headers.Get("User-Agent")
	acceptLang := headers.Get("Accept-Language")

	combined := userAgent + "\n" + acceptLang
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

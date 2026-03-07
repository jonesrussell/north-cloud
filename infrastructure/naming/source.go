package naming

import (
	"errors"
	"regexp"
	"strings"
)

const (
	// RawContentSuffix is the ES index suffix for unprocessed crawled content.
	RawContentSuffix = "_raw_content"
	// ClassifiedContentSuffix is the ES index suffix for classified content.
	ClassifiedContentSuffix = "_classified_content"
)

// invalidIndexChar matches characters NOT allowed in ES index names
// (anything other than lowercase alphanumeric and underscore).
var invalidIndexChar = regexp.MustCompile(`[^a-z0-9_]`)

// collapseUnderscores replaces runs of multiple underscores with a single one.
var collapseUnderscores = regexp.MustCompile(`_+`)

// SanitizeSourceName converts a human-readable source name into a valid
// Elasticsearch index prefix. It lowercases, replaces non-alphanumeric
// characters with underscores, collapses runs, and trims edges.
// Returns "" for empty input — callers decide the fallback.
func SanitizeSourceName(name string) string {
	if name == "" {
		return ""
	}

	s := strings.ToLower(strings.TrimSpace(name))
	s = invalidIndexChar.ReplaceAllString(s, "_")
	s = collapseUnderscores.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")

	return s
}

// RawContentIndex returns the raw_content ES index name for a source.
// Example: "Campbell River Mirror" → "campbell_river_mirror_raw_content"
func RawContentIndex(sourceName string) string {
	return SanitizeSourceName(sourceName) + RawContentSuffix
}

// ClassifiedContentIndex returns the classified_content ES index name for a source.
// Example: "Campbell River Mirror" → "campbell_river_mirror_classified_content"
func ClassifiedContentIndex(sourceName string) string {
	return SanitizeSourceName(sourceName) + ClassifiedContentSuffix
}

// ClassifiedIndexFromRaw converts a raw_content index name to its
// classified_content counterpart by swapping the suffix.
func ClassifiedIndexFromRaw(rawIndex string) (string, error) {
	if !strings.HasSuffix(rawIndex, RawContentSuffix) {
		return "", errors.New("invalid raw_content index name: missing suffix")
	}

	return rawIndex[:len(rawIndex)-len(RawContentSuffix)] + ClassifiedContentSuffix, nil
}

// IsRawContentIndex reports whether an index name has the raw_content suffix.
func IsRawContentIndex(name string) bool {
	return strings.HasSuffix(name, RawContentSuffix)
}

// IsClassifiedContentIndex reports whether an index name has the classified_content suffix.
func IsClassifiedContentIndex(name string) bool {
	return strings.HasSuffix(name, ClassifiedContentSuffix)
}

// BaseSourceFromIndex strips the raw_content or classified_content suffix
// and returns the sanitized source prefix. Returns an error if the index
// name does not end with a recognised suffix.
func BaseSourceFromIndex(indexName string) (string, error) {
	switch {
	case strings.HasSuffix(indexName, RawContentSuffix):
		return strings.TrimSuffix(indexName, RawContentSuffix), nil
	case strings.HasSuffix(indexName, ClassifiedContentSuffix):
		return strings.TrimSuffix(indexName, ClassifiedContentSuffix), nil
	default:
		return "", errors.New("index name does not have a recognised suffix")
	}
}

// Package feed provides parsers for sitemap XML and sitemap index formats.
// It extracts URLs from standard sitemaps and discovers child sitemaps from
// sitemap index files, with optional filtering by lastmod age.
package feed

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// dateOnlyFormat is the date-only layout for sitemap lastmod values (e.g. "2024-01-15").
const dateOnlyFormat = "2006-01-02"

// SitemapURL represents a single URL entry extracted from a sitemap.
type SitemapURL struct {
	Loc     string     `json:"loc"`
	LastMod *time.Time `json:"lastmod,omitempty"`
}

// xmlURLSet is the root element of a standard sitemap XML file.
type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []xmlURL `xml:"url"`
}

// xmlURL is a single <url> entry inside a <urlset>.
type xmlURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// xmlSitemapIndex is the root element of a sitemap index XML file.
type xmlSitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Sitemaps []xmlSitemap `xml:"sitemap"`
}

// xmlSitemap is a single <sitemap> entry inside a <sitemapindex>.
type xmlSitemap struct {
	Loc string `xml:"loc"`
}

// ParseSitemap parses sitemap XML and returns the contained URLs.
// When maxAge is greater than zero, only URLs whose lastmod is within
// maxAge of the current time are returned. URLs without a lastmod value
// are always included regardless of maxAge.
func ParseSitemap(body string, maxAge time.Duration) ([]SitemapURL, error) {
	var urlset xmlURLSet
	if err := xml.Unmarshal([]byte(body), &urlset); err != nil {
		return nil, fmt.Errorf("parse sitemap: %w", err)
	}

	cutoff := computeCutoff(maxAge)

	return buildSitemapURLs(urlset.URLs, cutoff), nil
}

// computeCutoff returns the earliest acceptable time when maxAge > 0,
// or a zero time when no filtering should be applied.
func computeCutoff(maxAge time.Duration) time.Time {
	if maxAge <= 0 {
		return time.Time{}
	}

	return time.Now().Add(-maxAge)
}

// buildSitemapURLs converts raw XML URL entries into SitemapURL values,
// applying an optional cutoff filter based on lastmod.
func buildSitemapURLs(xmlURLs []xmlURL, cutoff time.Time) []SitemapURL {
	result := make([]SitemapURL, 0, len(xmlURLs))

	for i := range xmlURLs {
		entry := &xmlURLs[i]
		su := convertXMLURL(entry)

		if shouldInclude(su.LastMod, cutoff) {
			result = append(result, su)
		}
	}

	return result
}

// convertXMLURL converts a raw XML URL entry into a SitemapURL, parsing
// the lastmod date if present.
func convertXMLURL(entry *xmlURL) SitemapURL {
	su := SitemapURL{Loc: entry.Loc}

	if entry.LastMod != "" {
		if t, err := parseLastMod(entry.LastMod); err == nil {
			su.LastMod = &t
		}
	}

	return su
}

// shouldInclude returns true if the URL should be included given the cutoff.
// URLs without a lastmod are always included. When cutoff is zero (no filtering),
// all URLs are included.
func shouldInclude(lastMod *time.Time, cutoff time.Time) bool {
	if cutoff.IsZero() {
		return true
	}

	if lastMod == nil {
		return true
	}

	return !lastMod.Before(cutoff)
}

// ParseSitemapIndex parses a sitemap index XML file and returns the
// URLs of all child sitemaps listed within it.
func ParseSitemapIndex(body string) ([]string, error) {
	var index xmlSitemapIndex
	if err := xml.Unmarshal([]byte(body), &index); err != nil {
		return nil, fmt.Errorf("parse sitemap index: %w", err)
	}

	urls := make([]string, 0, len(index.Sitemaps))
	for _, s := range index.Sitemaps {
		urls = append(urls, s.Loc)
	}

	return urls, nil
}

// parseLastMod attempts to parse a sitemap lastmod value. It tries RFC 3339
// first (e.g. "2024-01-15T10:30:00Z"), then falls back to the date-only
// format (e.g. "2024-01-15").
func parseLastMod(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)

	t, err := time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return t, nil
	}

	t, dateErr := time.Parse(dateOnlyFormat, trimmed)
	if dateErr == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("parse lastmod %q: %w", trimmed, dateErr)
}

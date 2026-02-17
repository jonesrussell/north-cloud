package feed_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

const (
	hoursPerDay       = 24
	daysInFilterRange = 30
	daysOld           = 90
)

// validSitemapXML is a fixture with 3 URLs for standard parsing tests.
const validSitemapXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page1</loc><lastmod>2024-06-15T10:00:00Z</lastmod></url>
  <url><loc>https://example.com/page2</loc><lastmod>2024-06-16T12:00:00Z</lastmod></url>
  <url><loc>https://example.com/page3</loc><lastmod>2024-06-17T14:00:00Z</lastmod></url>
</urlset>`

// sitemapIndexXML is a fixture with 2 child sitemaps.
const sitemapIndexXML = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>https://example.com/sitemap-news.xml</loc></sitemap>
  <sitemap><loc>https://example.com/sitemap-archive.xml</loc></sitemap>
</sitemapindex>`

// dateOnlySitemapXML uses the date-only lastmod format.
const dateOnlySitemapXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/article</loc><lastmod>2024-06-15</lastmod></url>
</urlset>`

// emptySitemapXML is a valid sitemap with no URL entries.
const emptySitemapXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
</urlset>`

const invalidSitemapXML = `<not valid xml<<<`

func TestParseSitemap(t *testing.T) {
	t.Parallel()

	urls, err := feed.ParseSitemap(validSitemapXML, 0)
	requireNoError(t, err)

	expectedCount := 3
	requireLen(t, urls, expectedCount)
	requireLoc(t, urls[0], "https://example.com/page1")
	requireLoc(t, urls[1], "https://example.com/page2")
	requireLoc(t, urls[2], "https://example.com/page3")

	requireLastModNotNil(t, urls[0])
	requireLastModNotNil(t, urls[1])
	requireLastModNotNil(t, urls[2])
}

func TestParseSitemapWithMaxAgeFilter(t *testing.T) {
	t.Parallel()

	recentTime := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	oldTime := time.Now().
		Add(-time.Duration(daysOld) * hoursPerDay * time.Hour).
		UTC().Format(time.RFC3339)

	sitemapBody := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/recent</loc><lastmod>` + recentTime + `</lastmod></url>
  <url><loc>https://example.com/old</loc><lastmod>` + oldTime + `</lastmod></url>
  <url><loc>https://example.com/no-date</loc></url>
</urlset>`

	maxAge := time.Duration(daysInFilterRange) * hoursPerDay * time.Hour
	urls, err := feed.ParseSitemap(sitemapBody, maxAge)
	requireNoError(t, err)

	// recent URL and no-date URL should be included; old URL should be filtered out
	expectedCount := 2
	requireLen(t, urls, expectedCount)
	requireLoc(t, urls[0], "https://example.com/recent")
	requireLoc(t, urls[1], "https://example.com/no-date")
}

func TestParseSitemapIndex(t *testing.T) {
	t.Parallel()

	urls, err := feed.ParseSitemapIndex(sitemapIndexXML)
	requireNoError(t, err)

	expectedCount := 2
	requireLen(t, urls, expectedCount)

	assertEqual(t, "https://example.com/sitemap-news.xml", urls[0])
	assertEqual(t, "https://example.com/sitemap-archive.xml", urls[1])
}

func TestParseSitemapEmpty(t *testing.T) {
	t.Parallel()

	urls, err := feed.ParseSitemap(emptySitemapXML, 0)
	requireNoError(t, err)

	if urls == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}

	requireLen(t, urls, 0)
}

func TestParseSitemapInvalidXML(t *testing.T) {
	t.Parallel()

	_, err := feed.ParseSitemap(invalidSitemapXML, 0)
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

func TestParseSitemapIndexInvalidXML(t *testing.T) {
	t.Parallel()

	_, err := feed.ParseSitemapIndex(invalidSitemapXML)
	if err == nil {
		t.Fatal("expected error for invalid XML, got nil")
	}
}

func TestParseSitemapDateOnlyLastmod(t *testing.T) {
	t.Parallel()

	urls, err := feed.ParseSitemap(dateOnlySitemapXML, 0)
	requireNoError(t, err)

	expectedCount := 1
	requireLen(t, urls, expectedCount)
	requireLastModNotNil(t, urls[0])

	expectedYear := 2024
	expectedMonth := time.June
	expectedDay := 15

	lm := *urls[0].LastMod
	if lm.Year() != expectedYear || lm.Month() != expectedMonth || lm.Day() != expectedDay {
		t.Errorf("expected 2024-06-15, got %s", lm.Format("2006-01-02"))
	}
}

// requireLoc fails the test if the SitemapURL Loc does not match expected.
func requireLoc(t *testing.T, su feed.SitemapURL, expected string) {
	t.Helper()

	if su.Loc != expected {
		t.Errorf("expected Loc %q, got %q", expected, su.Loc)
	}
}

// requireLastModNotNil fails the test if LastMod is nil.
func requireLastModNotNil(t *testing.T, su feed.SitemapURL) {
	t.Helper()

	if su.LastMod == nil {
		t.Fatalf("expected LastMod to be set for %s, got nil", su.Loc)
	}
}

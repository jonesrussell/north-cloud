package scraper

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
)

const fetchTimeout = 30 * time.Second

// PageFetcher downloads pages and extracts links and text.
type PageFetcher struct {
	httpClient *http.Client
}

// NewPageFetcher creates a new PageFetcher.
func NewPageFetcher() *PageFetcher {
	return &PageFetcher{
		httpClient: &http.Client{Timeout: fetchTimeout},
	}
}

// FetchLinks fetches a URL and returns all anchor links found on the page.
func (f *PageFetcher) FetchLinks(ctx context.Context, pageURL string) ([]leadership.Link, error) {
	doc, err := f.fetchDocument(ctx, pageURL)
	if err != nil {
		return nil, err
	}

	var links []leadership.Link

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		text := strings.TrimSpace(s.Text())
		links = append(links, leadership.Link{
			Href: href,
			Text: text,
		})
	})

	return links, nil
}

// FetchText fetches a URL and returns the visible text content of the page body.
func (f *PageFetcher) FetchText(ctx context.Context, pageURL string) (string, error) {
	doc, err := f.fetchDocument(ctx, pageURL)
	if err != nil {
		return "", err
	}

	// Remove script and style elements before extracting text
	doc.Find("script, style, noscript").Remove()

	text := doc.Find("body").Text()

	return NormalizeWhitespace(text), nil
}

// fetchDocument fetches a URL and parses it as an HTML document.
func (f *PageFetcher) fetchDocument(ctx context.Context, pageURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", pageURL, err)
	}

	resp, doErr := f.httpClient.Do(req)
	if doErr != nil {
		return nil, fmt.Errorf("fetch %s: %w", pageURL, doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", pageURL, resp.StatusCode)
	}

	doc, parseErr := goquery.NewDocumentFromReader(resp.Body)
	if parseErr != nil {
		return nil, fmt.Errorf("parse HTML from %s: %w", pageURL, parseErr)
	}

	return doc, nil
}

// NormalizeWhitespace collapses runs of horizontal whitespace into single spaces,
// preserves newlines for line-based text extraction, and trims.
func NormalizeWhitespace(s string) string {
	var b strings.Builder

	inSpace := false

	for _, r := range s {
		if r == '\n' || r == '\r' {
			inSpace = false
			b.WriteRune('\n')
			continue
		}

		if r == ' ' || r == '\t' {
			if !inSpace {
				b.WriteRune(' ')
				inSpace = true
			}
			continue
		}

		inSpace = false
		b.WriteRune(r)
	}

	return strings.TrimSpace(b.String())
}

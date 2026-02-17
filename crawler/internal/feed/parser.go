// Package feed provides RSS and Atom feed parsing for URL discovery.
package feed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// httpPrefix is the scheme prefix used to determine if a GUID is a valid URL.
const httpPrefix = "http"

// FeedItem represents a single entry extracted from an RSS or Atom feed.
type FeedItem struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
}

// ParseFeed parses an RSS or Atom feed body and returns the discovered items.
// Items without a usable link are silently skipped. An empty feed returns a
// non-nil empty slice.
func ParseFeed(ctx context.Context, body string) ([]FeedItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	parser := gofeed.NewParser()

	parsed, err := parser.ParseString(body)
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	items := make([]FeedItem, 0, len(parsed.Items))

	for _, entry := range parsed.Items {
		link := extractLink(entry)
		if link == "" {
			continue
		}

		item := FeedItem{
			URL:         link,
			Title:       entry.Title,
			PublishedAt: formatPublishedAt(entry.PublishedParsed),
		}

		items = append(items, item)
	}

	return items, nil
}

// extractLink returns the best available URL from a feed entry.
// It prefers the explicit Link field, falling back to the GUID if it
// looks like an HTTP URL.
func extractLink(entry *gofeed.Item) string {
	if entry.Link != "" {
		return entry.Link
	}

	if strings.HasPrefix(entry.GUID, httpPrefix) {
		return entry.GUID
	}

	return ""
}

// formatPublishedAt converts a parsed time pointer to an RFC3339 string.
// Returns an empty string when the time is nil.
func formatPublishedAt(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}

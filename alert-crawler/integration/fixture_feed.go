//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// Item holds the fields used to build one RSS <item> in a fixture feed.
type Item struct {
	Title       string
	Link        string
	PubDate     string
	Description string
}

// defaultPubDate is the RFC1123Z timestamp used when Item.PubDate is empty.
const defaultPubDate = "Mon, 06 Jan 2025 12:00:00 -0600"

// fixtureBaseYear, fixtureBaseMonth, etc. are the components of the stable
// base time used by FixturePubDate to generate unique per-item timestamps.
const (
	fixtureBaseYear   = 2025
	fixtureBaseMonth  = time.January
	fixtureBaseDay    = 6
	fixtureBaseHour   = 12
	fixtureCSTPOffset = -6 * 60 * 60 // seconds west of UTC
)

// BuildRSS returns a minimal RSS 2.0 document containing the supplied items.
// If an item's PubDate is empty, defaultPubDate is used.
func BuildRSS(items []Item) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<rss version="2.0"><channel>`)
	sb.WriteString(`<title>Fixture Feed</title>`)

	for _, it := range items {
		pubDate := it.PubDate
		if pubDate == "" {
			pubDate = defaultPubDate
		}

		sb.WriteString("<item>")
		fmt.Fprintf(&sb, "<title>%s</title>", xmlEscape(it.Title))
		fmt.Fprintf(&sb, "<link>%s</link>", it.Link)
		fmt.Fprintf(&sb, "<pubDate>%s</pubDate>", pubDate)
		fmt.Fprintf(&sb, "<description>%s</description>", xmlEscape(it.Description))
		sb.WriteString("</item>")
	}

	sb.WriteString(`</channel></rss>`)

	return sb.String()
}

// MutableServer is an httptest.Server whose response body can be swapped
// between poll rounds. Callers swap the body via SetBody.
type MutableServer struct {
	mu     sync.RWMutex
	body   string
	Server *httptest.Server
}

// NewMutableServer starts an httptest server that serves the initial body.
func NewMutableServer(initial string) *MutableServer {
	ms := &MutableServer{body: initial}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ms.mu.RLock()
		b := ms.body
		ms.mu.RUnlock()

		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, b)
	}))

	return ms
}

// SetBody atomically replaces the RSS body served by the test server.
func (ms *MutableServer) SetBody(body string) {
	ms.mu.Lock()
	ms.body = body
	ms.mu.Unlock()
}

// Close shuts down the underlying httptest server.
func (ms *MutableServer) Close() {
	ms.Server.Close()
}

// FixturePubDate returns a RFC1123Z timestamp for the given offset from a
// stable base time, so each item in a multi-item fixture gets a unique date.
func FixturePubDate(offset time.Duration) string {
	base := time.Date(
		fixtureBaseYear, fixtureBaseMonth, fixtureBaseDay,
		fixtureBaseHour, 0, 0, 0,
		time.FixedZone("CST", fixtureCSTPOffset),
	)

	return base.Add(offset).Format(time.RFC1123Z)
}

// xmlEscape replaces the five XML special characters so item strings do not
// break the RSS envelope.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")

	return s
}

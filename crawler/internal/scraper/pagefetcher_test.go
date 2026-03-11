package scraper_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
)

const (
	expectedLinkCount = 2
)

func TestPageFetcher_FetchLinks(t *testing.T) {
	html := `<html><body>
		<a href="/chief-and-council">Chief and Council</a>
		<a href="/contact-us">Contact Us</a>
		<a href="">Empty</a>
		<a>No href</a>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	fetcher := scraper.NewPageFetcher()

	links, err := fetcher.FetchLinks(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get 2 links (empty href and no-href are skipped)
	if len(links) != expectedLinkCount {
		t.Fatalf("expected %d links, got %d", expectedLinkCount, len(links))
	}

	if links[0].Href != "/chief-and-council" {
		t.Errorf("expected /chief-and-council, got %s", links[0].Href)
	}

	if links[0].Text != "Chief and Council" {
		t.Errorf("expected 'Chief and Council', got %s", links[0].Text)
	}

	if links[1].Href != "/contact-us" {
		t.Errorf("expected /contact-us, got %s", links[1].Href)
	}
}

func TestPageFetcher_FetchText(t *testing.T) {
	html := `<html><body>
		<script>var x = 1;</script>
		<style>body { color: red; }</style>
		<h1>Chief and Council</h1>
		<p>Chief John Smith leads the community.</p>
		<p>Councillor Jane Doe serves on council.</p>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	fetcher := scraper.NewPageFetcher()

	text, err := fetcher.FetchText(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not contain script or style content
	if strings.Contains(text, "var x = 1") {
		t.Error("text should not contain script content")
	}

	if strings.Contains(text, "color: red") {
		t.Error("text should not contain style content")
	}

	// Should contain actual content
	if !strings.Contains(text, "Chief and Council") {
		t.Error("text should contain 'Chief and Council'")
	}

	if !strings.Contains(text, "Chief John Smith") {
		t.Error("text should contain 'Chief John Smith'")
	}
}

func TestPageFetcher_FetchLinks_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	fetcher := scraper.NewPageFetcher()

	_, err := fetcher.FetchLinks(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	input := "  hello   \n\n  world  \t  foo  "
	expected := "hello \n\n world foo"

	result := scraper.NormalizeWhitespace(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

package leadership_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
)

func TestClassifyURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected leadership.PageType
	}{
		{"leadership path", "https://example.com/chief-and-council", leadership.PageTypeLeadership},
		{"leadership nested", "https://example.com/about/leadership", leadership.PageTypeLeadership},
		{"contact path", "https://example.com/contact-us", leadership.PageTypeContact},
		{"band office", "https://example.com/band-office", leadership.PageTypeContact},
		{"no match", "https://example.com/news/article-1", ""},
		{"governance", "https://example.com/governance", leadership.PageTypeLeadership},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leadership.ClassifyURL(tt.url)
			if got != tt.expected {
				t.Errorf("ClassifyURL(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

func TestClassifyLinkText(t *testing.T) {
	tests := []struct {
		text     string
		expected leadership.PageType
	}{
		{"Chief and Council", leadership.PageTypeLeadership},
		{"Contact Us", leadership.PageTypeContact},
		{"Band Office", leadership.PageTypeContact},
		{"News", ""},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := leadership.ClassifyLinkText(tt.text)
			if got != tt.expected {
				t.Errorf("ClassifyLinkText(%q) = %q, want %q", tt.text, got, tt.expected)
			}
		})
	}
}

func TestDiscoverPages(t *testing.T) {
	links := []leadership.Link{
		{Href: "/chief-and-council", Text: "Chief and Council"},
		{Href: "/contact", Text: "Contact"},
		{Href: "/news", Text: "News"},
		{Href: "/about", Text: "About"},
		{Href: "https://other.com/leadership", Text: "Partner"}, // different host
	}

	pages := leadership.DiscoverPages("https://example.com", links)

	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	if pages[0].PageType != leadership.PageTypeLeadership {
		t.Errorf("first page type = %q, want %q", pages[0].PageType, leadership.PageTypeLeadership)
	}

	if pages[1].PageType != leadership.PageTypeContact {
		t.Errorf("second page type = %q, want %q", pages[1].PageType, leadership.PageTypeContact)
	}
}

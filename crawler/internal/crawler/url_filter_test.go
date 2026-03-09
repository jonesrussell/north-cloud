package crawler_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
)

func TestShouldSkipURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		sourceHost string
		want       bool
	}{
		// Binary and asset files
		{"binary pdf", "https://example.com/report.pdf", "", true},
		{"binary image", "https://example.com/photo.jpg", "", true},
		{"css file", "https://example.com/style.css", "", true},
		// Non-content path segments
		{"login page", "https://example.com/login", "", true},
		{"wp-admin", "https://example.com/wp-admin/edit.php", "", true},
		{"cart page", "https://example.com/cart", "", true},
		{"shop page", "https://example.com/shop/item-123", "", true},
		{"product page", "https://example.com/products/widget", "", true},
		{"store page", "https://example.com/store/checkout", "", true},
		{"category page", "https://example.com/category/sports", "", true},
		{"tag page", "https://example.com/tag/breaking-news", "", true},
		// CDN asset paths
		{"wp uploads binary", "https://example.com/wp-content/uploads/2026/photo.jpg", "", true},
		{"wp uploads non-binary", "https://example.com/wp-content/uploads/2026/doc.html", "", true},
		{"assets path binary", "https://example.com/assets/images/logo.png", "", true},
		{"assets path non-binary", "https://example.com/assets/data/config.json", "", true},
		{"static path non-binary", "https://example.com/static/js/app.js", "", true},
		// Known non-content hosts
		{"google play", "https://play.google.com/store/apps/details?id=com.example", "", true},
		{"apple app store", "https://apps.apple.com/us/app/example/id123456", "", true},
		{"cloudfront cdn", "https://d1abc123.cloudfront.net/assets/image.jpg", "", true},
		{"fbcdn image", "https://static.xx.fbcdn.net/rsrc.php/v4/y1/r/image.png", "", true},
		// Content URLs (no sourceHost — off-domain check disabled)
		{"article page", "https://example.com/news/2026/03/headline-here", "", false},
		{"homepage", "https://example.com/", "", false},
		{"about page", "https://example.com/about", "", true},
		{"normal article", "https://example.com/story/some-article-title", "", false},
		// Off-domain filtering (sourceHost set)
		{"same host allowed", "https://example.com/news/article", "example.com", false},
		{"off-domain skipped", "https://other.com/news/article", "example.com", true},
		{"subdomain skipped when strict", "https://cdn.example.com/image.jpg", "example.com", true},
		{"off-domain social link", "https://twitter.com/user/status/1", "example.com", true},
		{"empty sourceHost no filter", "https://other.com/news/article", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crawler.ShouldSkipURL(tt.url, tt.sourceHost)
			if got != tt.want {
				t.Errorf("shouldSkipURL(%q, %q) = %v, want %v", tt.url, tt.sourceHost, got, tt.want)
			}
		})
	}
}

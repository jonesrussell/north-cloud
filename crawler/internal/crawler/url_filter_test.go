package crawler_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
)

func TestShouldSkipURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"binary pdf", "https://example.com/report.pdf", true},
		{"binary image", "https://example.com/photo.jpg", true},
		{"css file", "https://example.com/style.css", true},
		{"login page", "https://example.com/login", true},
		{"wp-admin", "https://example.com/wp-admin/edit.php", true},
		{"cart page", "https://example.com/cart", true},
		{"shop page", "https://example.com/shop/item-123", true},
		{"product page", "https://example.com/products/widget", true},
		{"store page", "https://example.com/store/checkout", true},
		{"category page", "https://example.com/category/sports", true},
		{"tag page", "https://example.com/tag/breaking-news", true},
		{"wp uploads", "https://example.com/wp-content/uploads/2026/photo.jpg", true},
		{"assets path", "https://example.com/assets/images/logo.png", true},
		{"article page", "https://example.com/news/2026/03/headline-here", false},
		{"homepage", "https://example.com/", false},
		{"about page", "https://example.com/about", true},
		{"normal article", "https://example.com/story/some-article-title", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crawler.ShouldSkipURL(tt.url)
			if got != tt.want {
				t.Errorf("shouldSkipURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

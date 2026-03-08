package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

func TestClassifyPageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		title     string
		wordCount int
		linkCount int
		want      string
	}{
		{"article with content", "Breaking News", 350, 5, rawcontent.PageTypeArticle},
		{"article at threshold", "Story", 200, 3, rawcontent.PageTypeArticle},
		{"stub with title", "Headline", 20, 2, rawcontent.PageTypeStub},
		{"stub empty body", "Title Only", 0, 1, rawcontent.PageTypeStub},
		{"listing page", "", 100, 30, rawcontent.PageTypeListing},
		{"listing high link ratio", "News", 50, 25, rawcontent.PageTypeListing},
		{"other no title", "", 100, 5, rawcontent.PageTypeOther},
		{"other low content", "", 80, 8, rawcontent.PageTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rawcontent.ClassifyPageType(tt.title, tt.wordCount, tt.linkCount)
			if got != tt.want {
				t.Errorf("ClassifyPageType(%q, %d, %d) = %q, want %q",
					tt.title, tt.wordCount, tt.linkCount, got, tt.want)
			}
		})
	}
}

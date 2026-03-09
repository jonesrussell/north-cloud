package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

func TestClassifyPageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		title               string
		wordCount           int
		linkCount           int
		ogType              string
		detectedContentType string
		jsonLDType          string
		articleTagCount     int
		hasDateTime         bool
		hasSignInText       bool
		want                string
	}{
		// Word count + title (score=4 from scoreWordCountTitle)
		{name: "article high word count", title: "Breaking News", wordCount: 350, want: rawcontent.PageTypeArticle},
		{name: "article at word threshold", title: "Story", wordCount: 200, want: rawcontent.PageTypeArticle},
		// OG type combined with word count
		{name: "article og type and word count", title: "Event", wordCount: 250, ogType: "article", want: rawcontent.PageTypeArticle},
		// JSON-LD signals alone exceed threshold (score=5)
		{name: "article newsarticle jsonld", title: "Piece", wordCount: 50, jsonLDType: "NewsArticle", want: rawcontent.PageTypeArticle},
		{name: "article blogposting jsonld", jsonLDType: "BlogPosting", want: rawcontent.PageTypeArticle},
		// HTML structural signals
		{name: "article tag plus word count", title: "News", wordCount: 200, articleTagCount: 1, want: rawcontent.PageTypeArticle},
		{
			name: "article tag plus datetime via jsonld", title: "Piece",
			jsonLDType: "Article", articleTagCount: 2, hasDateTime: true,
			want: rawcontent.PageTypeArticle,
		},
		// Sign-in override always returns other
		{name: "sign-in page overrides article signals", title: "Login", wordCount: 500, hasSignInText: true, want: rawcontent.PageTypeOther},
		// Stub: has title, word count below threshold
		{name: "stub with title", title: "Headline", wordCount: 20, want: rawcontent.PageTypeStub},
		{name: "stub empty body", title: "Title Only", wordCount: 0, want: rawcontent.PageTypeStub},
		// Listing: many links, low word-per-link ratio
		{name: "listing page", wordCount: 100, linkCount: 30, want: rawcontent.PageTypeListing},
		{name: "listing high link ratio", title: "News", wordCount: 50, linkCount: 25, want: rawcontent.PageTypeListing},
		// Other: insufficient signals for any positive classification
		{name: "other no title", wordCount: 100, linkCount: 5, want: rawcontent.PageTypeOther},
		{name: "other low content", wordCount: 80, linkCount: 8, want: rawcontent.PageTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rawcontent.ClassifyPageType(rawcontent.MakeSignals(
				tt.title,
				tt.wordCount, tt.linkCount,
				tt.ogType, tt.detectedContentType, tt.jsonLDType,
				tt.articleTagCount,
				tt.hasDateTime, tt.hasSignInText,
			))
			if got != tt.want {
				t.Errorf("classifyPageType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractHTMLSignals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		html              string
		wantArticleTags   int
		wantHasDateTime   bool
		wantHasSignInText bool
	}{
		{
			name:            "no signals",
			html:            "<p>Some content here</p>",
			wantArticleTags: 0, wantHasDateTime: false, wantHasSignInText: false,
		},
		{
			name:            "single article tag",
			html:            `<article class="post"><p>Content</p></article>`,
			wantArticleTags: 1, wantHasDateTime: false, wantHasSignInText: false,
		},
		{
			name:            "multiple article tags",
			html:            `<article>A</article><article>B</article>`,
			wantArticleTags: 2, wantHasDateTime: false, wantHasSignInText: false,
		},
		{
			name:            "datetime attribute",
			html:            `<time datetime="2026-01-15">Jan 15</time>`,
			wantArticleTags: 0, wantHasDateTime: true, wantHasSignInText: false,
		},
		{
			name:            "sign in text",
			html:            `<a href="/login">Sign In</a>`,
			wantArticleTags: 0, wantHasDateTime: false, wantHasSignInText: true,
		},
		{
			name:            "log in text",
			html:            `<button>Log in to continue</button>`,
			wantArticleTags: 0, wantHasDateTime: false, wantHasSignInText: true,
		},
		{
			name:            "sign-in hyphenated",
			html:            `<div class="sign-in-wall">Subscribe</div>`,
			wantArticleTags: 0, wantHasDateTime: false, wantHasSignInText: true,
		},
		{
			name:              "all signals present",
			html:              `<article><time datetime="2026-03-01">Mar 1</time><p>Content</p></article><a>Sign in</a>`,
			wantArticleTags:   1,
			wantHasDateTime:   true,
			wantHasSignInText: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotTags, gotDateTime, gotSignIn := rawcontent.ExtractHTMLSignals(tt.html)
			if gotTags != tt.wantArticleTags {
				t.Errorf("articleTagCount = %d, want %d", gotTags, tt.wantArticleTags)
			}
			if gotDateTime != tt.wantHasDateTime {
				t.Errorf("hasDateTime = %v, want %v", gotDateTime, tt.wantHasDateTime)
			}
			if gotSignIn != tt.wantHasSignInText {
				t.Errorf("hasSignInText = %v, want %v", gotSignIn, tt.wantHasSignInText)
			}
		})
	}
}

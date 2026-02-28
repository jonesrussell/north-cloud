package bootstrap_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

func TestMapExtractedToRawContent_AllFields(t *testing.T) {
	t.Parallel()

	content := &fetcher.ExtractedContent{
		Title:         "Test Article",
		Body:          "Some body text here",
		Description:   "A test description",
		Author:        "Jane Doe",
		ContentHash:   "abc123hash",
		URL:           "https://example.com/article",
		SourceID:      "source-1",
		OGType:        "article",
		OGTitle:       "OG Test Title",
		OGDescription: "OG test description",
		OGImage:       "https://example.com/image.jpg",
		CanonicalURL:  "https://example.com/canonical",
		MetaKeywords:  "news, test",
		PublishedDate: "2025-06-15T10:30:00Z",
		WordCount:     4,
	}

	rc := bootstrap.MapExtractedToRawContentForTest(content, "example.com", infralogger.NewNop())

	assertEqual(t, "ID", "abc123hash", rc.ID)
	assertEqual(t, "URL", "https://example.com/article", rc.URL)
	assertEqual(t, "SourceName", "example.com", rc.SourceName)
	assertEqual(t, "Title", "Test Article", rc.Title)
	assertEqual(t, "RawText", "Some body text here", rc.RawText)
	assertEqual(t, "MetaDescription", "A test description", rc.MetaDescription)
	assertEqual(t, "Author", "Jane Doe", rc.Author)
	assertEqual(t, "OGType", "article", rc.OGType)
	assertEqual(t, "OGTitle", "OG Test Title", rc.OGTitle)
	assertEqual(t, "OGDescription", "OG test description", rc.OGDescription)
	assertEqual(t, "OGImage", "https://example.com/image.jpg", rc.OGImage)
	assertEqual(t, "CanonicalURL", "https://example.com/canonical", rc.CanonicalURL)
	assertEqual(t, "MetaKeywords", "news, test", rc.MetaKeywords)
	assertEqual(t, "ClassificationStatus", "pending", rc.ClassificationStatus)
	assertIntEqual(t, "WordCount", 4, rc.WordCount)

	if rc.PublishedDate == nil {
		t.Fatal("expected PublishedDate to be non-nil")
	}
	expected := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	if !rc.PublishedDate.Equal(expected) {
		t.Errorf("PublishedDate: expected %v, got %v", expected, *rc.PublishedDate)
	}
	if rc.CrawledAt.IsZero() {
		t.Error("CrawledAt should not be zero")
	}
}

func TestMapExtractedToRawContent_EmptyPublishedDate(t *testing.T) {
	t.Parallel()

	content := &fetcher.ExtractedContent{
		ContentHash: "hash1",
		URL:         "https://example.com",
		SourceID:    "source-1",
	}

	rc := bootstrap.MapExtractedToRawContentForTest(content, "example.com", infralogger.NewNop())

	if rc.PublishedDate != nil {
		t.Errorf("expected nil PublishedDate for empty string, got %v", *rc.PublishedDate)
	}
}

func TestMapExtractedToRawContent_DateOnlyFormat(t *testing.T) {
	t.Parallel()

	content := &fetcher.ExtractedContent{
		ContentHash:   "hash2",
		URL:           "https://example.com",
		SourceID:      "source-1",
		PublishedDate: "2025-06-15",
	}

	rc := bootstrap.MapExtractedToRawContentForTest(content, "example.com", infralogger.NewNop())

	if rc.PublishedDate == nil {
		t.Fatal("expected PublishedDate to be parsed from date-only format, got nil")
	}
	expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if !rc.PublishedDate.Equal(expected) {
		t.Errorf("PublishedDate: expected %v, got %v", expected, *rc.PublishedDate)
	}
}

func TestMapExtractedToRawContent_UnparseableDate(t *testing.T) {
	t.Parallel()

	content := &fetcher.ExtractedContent{
		ContentHash:   "hash3",
		URL:           "https://example.com",
		SourceID:      "source-1",
		PublishedDate: "June 15, 2025",
	}

	rc := bootstrap.MapExtractedToRawContentForTest(content, "example.com", infralogger.NewNop())

	if rc.PublishedDate != nil {
		t.Errorf("expected nil PublishedDate for unparseable format, got %v", *rc.PublishedDate)
	}
}

func TestParsePublishedDate_Formats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantYMD [3]int // year, month, day
	}{
		{"RFC3339", "2025-06-15T10:30:00Z", true, [3]int{2025, 6, 15}},
		{"RFC3339 with offset", "2025-06-15T10:30:00+05:00", true, [3]int{2025, 6, 15}},
		{"datetime no timezone", "2025-06-15T10:30:00", true, [3]int{2025, 6, 15}},
		{"datetime with space", "2025-06-15 10:30:00", true, [3]int{2025, 6, 15}},
		{"date only", "2025-06-15", true, [3]int{2025, 6, 15}},
		{"human readable", "June 15, 2025", false, [3]int{}},
		{"empty", "", false, [3]int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, ok := bootstrap.ParsePublishedDateForTest(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parsePublishedDate(%q): ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && tt.wantOK {
				if parsed.Year() != tt.wantYMD[0] ||
					int(parsed.Month()) != tt.wantYMD[1] ||
					parsed.Day() != tt.wantYMD[2] {
					t.Errorf("parsePublishedDate(%q): got %v, want year=%d month=%d day=%d",
						tt.input, parsed, tt.wantYMD[0], tt.wantYMD[1], tt.wantYMD[2])
				}
			}
		})
	}
}

// --- test helpers ---

func assertEqual(t *testing.T, field, expected, actual string) {
	t.Helper()

	if actual != expected {
		t.Errorf("%s: expected %q, got %q", field, expected, actual)
	}
}

func assertIntEqual(t *testing.T, field string, expected, actual int) {
	t.Helper()

	if actual != expected {
		t.Errorf("%s: expected %d, got %d", field, expected, actual)
	}
}

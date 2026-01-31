package worker_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/publisher/internal/worker"
)

func TestDefaultOutboxWorkerConfig(t *testing.T) {
	cfg := worker.DefaultOutboxWorkerConfig()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, 5*time.Second)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, 100)
	}
	if cfg.PublishTimeout != 10*time.Second {
		t.Errorf("PublishTimeout = %v, want %v", cfg.PublishTimeout, 10*time.Second)
	}
}

func TestOutboxWorkerConfig_Validation(t *testing.T) {
	// Test that default config has valid values
	defaultCfg := worker.DefaultOutboxWorkerConfig()
	if defaultCfg.PollInterval <= 0 {
		t.Error("default PollInterval should be positive")
	}
	if defaultCfg.BatchSize <= 0 {
		t.Error("default BatchSize should be positive")
	}
	if defaultCfg.PublishTimeout <= 0 {
		t.Error("default PublishTimeout should be positive")
	}
}

func TestOutboxEntry_RoutingKey(t *testing.T) {
	testCases := []struct {
		name    string
		entry   domain.OutboxEntry
		wantKey string
	}{
		{
			name: "crime with subcategory",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: strPtr("violent_crime"),
				ContentType:      "article",
			},
			wantKey: "articles:crime:violent_crime",
		},
		{
			name: "crime property",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: strPtr("property_crime"),
				ContentType:      "article",
			},
			wantKey: "articles:crime:property_crime",
		},
		{
			name: "crime drug",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: strPtr("drug_crime"),
				ContentType:      "article",
			},
			wantKey: "articles:crime:drug_crime",
		},
		{
			name: "crime organized",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: strPtr("organized_crime"),
				ContentType:      "article",
			},
			wantKey: "articles:crime:organized_crime",
		},
		{
			name: "crime justice",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: strPtr("criminal_justice"),
				ContentType:      "article",
			},
			wantKey: "articles:crime:criminal_justice",
		},
		{
			name: "crime without subcategory",
			entry: domain.OutboxEntry{
				IsCrimeRelated:   true,
				CrimeSubcategory: nil,
				ContentType:      "article",
			},
			wantKey: "articles:crime",
		},
		{
			name: "article content type",
			entry: domain.OutboxEntry{
				IsCrimeRelated: false,
				ContentType:    "article",
			},
			wantKey: "articles:news",
		},
		{
			name: "video content type",
			entry: domain.OutboxEntry{
				IsCrimeRelated: false,
				ContentType:    "video",
			},
			wantKey: "content:video",
		},
		{
			name: "image content type",
			entry: domain.OutboxEntry{
				IsCrimeRelated: false,
				ContentType:    "image",
			},
			wantKey: "content:image",
		},
		{
			name: "unknown content type",
			entry: domain.OutboxEntry{
				IsCrimeRelated: false,
				ContentType:    "unknown",
			},
			wantKey: "content:other",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey := tc.entry.RoutingKey()
			if gotKey != tc.wantKey {
				t.Errorf("RoutingKey() = %s, want %s", gotKey, tc.wantKey)
			}
		})
	}
}

func TestOutboxEntry_ToPublishMessage(t *testing.T) {
	now := time.Now()
	entry := domain.OutboxEntry{
		ID:            "outbox-123",
		ContentID:     "content-456",
		SourceName:    "test-source",
		IndexName:     "test_classified_content",
		ContentType:   "article",
		Topics:        []string{"news", "local"},
		QualityScore:  85,
		Title:         "Test Article",
		Body:          "This is the article body.",
		URL:           "https://example.com/article",
		PublishedDate: &now,
	}

	msg := entry.ToPublishMessage()

	// Verify publisher metadata
	publisher, ok := msg["publisher"].(map[string]any)
	if !ok {
		t.Fatal("publisher metadata not found or wrong type")
	}
	if publisher["outbox_id"] != entry.ID {
		t.Errorf("Publisher.OutboxID = %v, want %s", publisher["outbox_id"], entry.ID)
	}
	if publisher["channel"] != entry.RoutingKey() {
		t.Errorf("Publisher.Channel = %v, want %s", publisher["channel"], entry.RoutingKey())
	}

	// Verify content fields
	if msg["id"] != entry.ContentID {
		t.Errorf("id = %v, want %s", msg["id"], entry.ContentID)
	}
	if msg["title"] != entry.Title {
		t.Errorf("title = %v, want %s", msg["title"], entry.Title)
	}
	if msg["body"] != entry.Body {
		t.Errorf("body = %v, want %s", msg["body"], entry.Body)
	}
	if msg["quality_score"] != entry.QualityScore {
		t.Errorf("quality_score = %v, want %d", msg["quality_score"], entry.QualityScore)
	}
	if msg["source"] != entry.SourceName {
		t.Errorf("source = %v, want %s", msg["source"], entry.SourceName)
	}
}

// strPtr is a helper to create string pointers
func strPtr(s string) *string {
	return &s
}

// Benchmark tests for performance-critical paths
func BenchmarkOutboxEntry_RoutingKey(b *testing.B) {
	entry := domain.OutboxEntry{
		IsCrimeRelated:   true,
		CrimeSubcategory: strPtr("violent_crime"),
		ContentType:      "article",
	}

	b.ResetTimer()
	for range b.N {
		_ = entry.RoutingKey()
	}
}

func BenchmarkOutboxEntry_ToPublishMessage(b *testing.B) {
	now := time.Now()
	entry := domain.OutboxEntry{
		ID:            "outbox-123",
		ContentID:     "content-456",
		SourceName:    "test-source",
		IndexName:     "test_classified_content",
		ContentType:   "article",
		Topics:        []string{"news", "local"},
		QualityScore:  85,
		Title:         "Test Article Title That Is Reasonably Long",
		Body:          "This is the article body with some content that would be typical of a news article.",
		URL:           "https://example.com/article/path/to/content",
		PublishedDate: &now,
	}

	b.ResetTimer()
	for range b.N {
		_ = entry.ToPublishMessage()
	}
}

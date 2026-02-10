package domain_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// expectedURLHashShortLen is the number of hex characters returned by URLHashShort.
const expectedURLHashShortLen = 8

// testServiceName is the service name used across tests.
const testServiceName = "crawler"

// testArticleURL is the article URL used across tests.
const testArticleURL = "https://example.com/article/1"

func TestStage_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stage domain.Stage
		want  bool
	}{
		{name: "crawled is valid", stage: domain.StageCrawled, want: true},
		{name: "indexed is valid", stage: domain.StageIndexed, want: true},
		{name: "classified is valid", stage: domain.StageClassified, want: true},
		{name: "routed is valid", stage: domain.StageRouted, want: true},
		{name: "published is valid", stage: domain.StagePublished, want: true},
		{name: "empty is invalid", stage: domain.Stage(""), want: false},
		{name: "unknown is invalid", stage: domain.Stage("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.stage.IsValid(); got != tt.want {
				t.Errorf("Stage(%q).IsValid() = %v, want %v", tt.stage, got, tt.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{name: "simple URL", rawURL: "https://example.com/path", want: "example.com"},
		{name: "www prefix stripped", rawURL: "https://www.example.com/path", want: "example.com"},
		{name: "URL with port", rawURL: "https://example.com:8080/path", want: "example.com"},
		{name: "invalid URL", rawURL: "://bad", want: "unknown"},
		{name: "empty string", rawURL: "", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := domain.ExtractDomain(tt.rawURL)
			if got != tt.want {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestURLHashShort(t *testing.T) {
	t.Parallel()

	t.Run("length equals expected", func(t *testing.T) {
		t.Parallel()

		got := domain.URLHashShort(testArticleURL)
		if len(got) != expectedURLHashShortLen {
			t.Errorf("URLHashShort(%q) length = %d, want %d", testArticleURL, len(got), expectedURLHashShortLen)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		first := domain.URLHashShort(testArticleURL)
		second := domain.URLHashShort(testArticleURL)
		if first != second {
			t.Errorf("URLHashShort is not deterministic: %q != %q", first, second)
		}
	})

	t.Run("different URLs produce different hashes", func(t *testing.T) {
		t.Parallel()

		hashA := domain.URLHashShort("https://example.com/a")
		hashB := domain.URLHashShort("https://example.com/b")
		if hashA == hashB {
			t.Errorf("URLHashShort produced same hash for different URLs: %q", hashA)
		}
	})
}

func TestURLHash(t *testing.T) {
	t.Parallel()

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		first := domain.URLHash(testArticleURL)
		second := domain.URLHash(testArticleURL)
		if first != second {
			t.Errorf("URLHash is not deterministic: %q != %q", first, second)
		}
	})

	t.Run("short hash is prefix of full hash", func(t *testing.T) {
		t.Parallel()

		full := domain.URLHash(testArticleURL)
		short := domain.URLHashShort(testArticleURL)
		if full[:expectedURLHashShortLen] != short {
			t.Errorf("URLHashShort(%q) = %q, but URLHash starts with %q", testArticleURL, short, full[:expectedURLHashShortLen])
		}
	})
}

func TestGenerateIdempotencyKey(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)

	t.Run("non-empty", func(t *testing.T) {
		t.Parallel()

		key := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, fixedTime)
		if key == "" {
			t.Error("GenerateIdempotencyKey returned empty string")
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		t.Parallel()

		first := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, fixedTime)
		second := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, fixedTime)
		if first != second {
			t.Errorf("GenerateIdempotencyKey is not deterministic: %q != %q", first, second)
		}
	})

	t.Run("different service produces different key", func(t *testing.T) {
		t.Parallel()

		keyA := domain.GenerateIdempotencyKey("crawler", domain.StageCrawled, testArticleURL, fixedTime)
		keyB := domain.GenerateIdempotencyKey("classifier", domain.StageCrawled, testArticleURL, fixedTime)
		if keyA == keyB {
			t.Errorf("different services produced same key: %q", keyA)
		}
	})

	t.Run("different stage produces different key", func(t *testing.T) {
		t.Parallel()

		keyA := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, fixedTime)
		keyB := domain.GenerateIdempotencyKey(testServiceName, domain.StageClassified, testArticleURL, fixedTime)
		if keyA == keyB {
			t.Errorf("different stages produced same key: %q", keyA)
		}
	})

	t.Run("different URL produces different key", func(t *testing.T) {
		t.Parallel()

		keyA := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, "https://example.com/a", fixedTime)
		keyB := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, "https://example.com/b", fixedTime)
		if keyA == keyB {
			t.Errorf("different URLs produced same key: %q", keyA)
		}
	})

	t.Run("different time produces different key", func(t *testing.T) {
		t.Parallel()

		timeA := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)
		timeB := time.Date(2025, time.January, 2, 12, 0, 0, 0, time.UTC)
		keyA := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, timeA)
		keyB := domain.GenerateIdempotencyKey(testServiceName, domain.StageCrawled, testArticleURL, timeB)
		if keyA == keyB {
			t.Errorf("different times produced same key: %q", keyA)
		}
	})
}

func TestAllStages(t *testing.T) {
	t.Parallel()

	stages := domain.AllStages()

	const expectedStageCount = 5
	if len(stages) != expectedStageCount {
		t.Errorf("AllStages() returned %d stages, want %d", len(stages), expectedStageCount)
	}

	for _, s := range stages {
		if !s.IsValid() {
			t.Errorf("AllStages() returned invalid stage %q", s)
		}
	}
}

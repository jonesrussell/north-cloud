package handlers_test

import (
	"encoding/json"
	"testing"
	"time"
)

// Source represents a content source for benchmarking
type Source struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Enabled     bool              `json:"enabled"`
	Selectors   map[string]string `json:"selectors"`
	Metadata    map[string]any    `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastCrawled *time.Time        `json:"last_crawled,omitempty"`
}

// BenchmarkSourceCRUD benchmarks source CRUD operations
func BenchmarkSourceCRUD(b *testing.B) {
	// Create test source
	source := Source{
		ID:      "example_com",
		Name:    "Example News",
		URL:     "https://example.com",
		Enabled: true,
		Selectors: map[string]string{
			"article":        ".article",
			"title":          "h1",
			"content":        ".content",
			"published_date": "time",
		},
		Metadata: map[string]any{
			"category": "news",
			"region":   "local",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate create
		_, err := json.Marshal(source)
		if err != nil {
			b.Fatal(err)
		}

		// Simulate read (no-op for benchmark)
		_ = source.ID

		// Simulate update
		source.UpdatedAt = time.Now().UTC()
		source.Metadata["last_check"] = "updated"

		// Serialize updated source
		_, err = json.Marshal(source)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSourceValidation benchmarks source configuration validation
func BenchmarkSourceValidation(b *testing.B) {
	sources := []Source{
		{
			ID:      "valid_source",
			Name:    "Valid News Site",
			URL:     "https://example.com",
			Enabled: true,
			Selectors: map[string]string{
				"article": ".article",
				"title":   "h1",
			},
		},
		{
			ID:      "invalid_no_url",
			Name:    "Invalid Source",
			URL:     "",
			Enabled: true,
			Selectors: map[string]string{
				"article": ".article",
			},
		},
		{
			ID:        "invalid_no_selectors",
			Name:      "Another Invalid",
			URL:       "https://example.com",
			Enabled:   true,
			Selectors: map[string]string{},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for i := range sources {
			source := &sources[i]
			// Validate URL
			isValid := source.URL != "" && (len(source.URL) > 8)

			// Validate selectors
			if isValid {
				isValid = len(source.Selectors) > 0
			}

			// Validate ID
			if isValid {
				isValid = source.ID != ""
			}

			_ = isValid
		}
	}
}

// BenchmarkTestCrawl benchmarks test crawl simulation
func BenchmarkTestCrawl(b *testing.B) {
	source := Source{
		ID:  "example_com",
		URL: "https://example.com/news",
		Selectors: map[string]string{
			"article":        ".article",
			"title":          "h1.title",
			"content":        ".article-body",
			"published_date": "time.published",
			"author":         ".author-name",
		},
	}
	_ = source.ID
	_ = source.URL

	// Simulated HTML for test crawl
	html := `<html><body>
		<div class="article">
			<h1 class="title">Article Title</h1>
			<span class="author-name">John Doe</span>
			<time class="published">2026-01-03</time>
			<div class="article-body">Article content here</div>
		</div>
	</body></html>`

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate selector testing
		results := make(map[string]int)

		for selectorName := range source.Selectors {
			// Simulate finding elements (simplified)
			// In reality, this would use goquery
			results[selectorName] = 1
		}

		// Build test response
		response := map[string]any{
			"success":     true,
			"articles":    results,
			"total_found": len(results),
		}

		_, err := json.Marshal(response)
		if err != nil {
			b.Fatal(err)
		}

		_ = html // Prevent unused variable error
	}
}

// BenchmarkSelectorMatching benchmarks CSS selector matching
func BenchmarkSelectorMatching(b *testing.B) {
	selectors := map[string]string{
		"article":        ".article",
		"title":          "h1.title, h2.headline",
		"content":        ".content, .article-body, .post-content",
		"published_date": "time[datetime], .published, .date",
		"author":         ".author, .byline, .writer",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate selector validation and matching
		validSelectors := 0

		for _, selector := range selectors {
			// Check if selector is not empty
			if selector != "" {
				validSelectors++
			}
		}

		_ = validSelectors
	}
}

// BenchmarkMetadataExtraction benchmarks extracting metadata from sources
func BenchmarkMetadataExtraction(b *testing.B) {
	source := Source{
		ID:   "example_com",
		Name: "Example News",
		URL:  "https://example.com",
		Metadata: map[string]any{
			"category":      "news",
			"region":        "north-america",
			"language":      "en",
			"update_freq":   "hourly",
			"priority":      "high",
			"verified":      true,
			"contact_email": "news@example.com",
		},
	}
	_ = source.Name
	_ = source.URL

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Extract and process metadata
		metadata := make(map[string]any)

		// Copy all metadata
		for key, value := range source.Metadata {
			metadata[key] = value
		}

		// Add computed fields
		metadata["id"] = source.ID
		metadata["enabled"] = source.Enabled
		metadata["has_selectors"] = len(source.Selectors) > 0

		_, err := json.Marshal(metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSourceListing benchmarks listing and filtering sources
func BenchmarkSourceListing(b *testing.B) {
	// Create 100 test sources
	sources := make([]Source, 100)
	for i := range 100 {
		sources[i] = Source{
			ID:      "source_" + string(rune(i)),
			Name:    "Source Name",
			URL:     "https://example.com",
			Enabled: i%2 == 0,
			Selectors: map[string]string{
				"article": ".article",
			},
			CreatedAt: time.Now().AddDate(0, 0, -i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Filter enabled sources
		enabled := make([]Source, 0, 50)
		for i := range sources {
			source := &sources[i]
			if source.Enabled {
				enabled = append(enabled, *source)
			}
		}

		// Sort by created_at (simplified)
		_ = enabled
	}
}

// BenchmarkURLParsing benchmarks parsing and validating URLs
func BenchmarkURLParsing(b *testing.B) {
	urls := []string{
		"https://example.com/news",
		"http://news.example.org/articles",
		"https://www.local-news.com/breaking",
		"https://subdomain.example.co.uk/section/page",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, url := range urls {
			// Validate URL format (simplified)
			isValid := len(url) > 8 &&
				(url[:7] == "http://" || url[:8] == "https://")

			_ = isValid
		}
	}
}

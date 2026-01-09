package router_test

import (
	"encoding/json"
	"testing"
	"time"
)

// Article represents a classified article for benchmarking
type Article struct {
	SourceID       string         `json:"source_id"`
	URL            string         `json:"url"`
	CanonicalURL   string         `json:"canonical_url"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	PublishedDate  time.Time      `json:"published_date"`
	QualityScore   int            `json:"quality_score"`
	IsCrimeRelated bool           `json:"is_crime_related"`
	Topics         []string       `json:"topics"`
	ContentType    string         `json:"content_type"`
	Metadata       map[string]any `json:"metadata"`
	ClassifiedAt   time.Time      `json:"classified_at"`
}

// Route represents a routing rule for benchmarking
type Route struct {
	ID              int64
	SourceID        int64
	ChannelID       int64
	MinQualityScore int
	Topics          []string
	IsActive        bool
}

// BenchmarkArticleFiltering benchmarks route filtering logic
//
//nolint:gocognit // Benchmark function complexity is acceptable
func BenchmarkArticleFiltering(b *testing.B) {
	// Create 100 test articles with varying quality scores and topics
	articles := make([]Article, 100)
	for i := range 100 {
		topics := []string{"news"}
		isCrime := i%3 == 0
		if isCrime {
			topics = append(topics, "crime")
		}

		articles[i] = Article{
			SourceID:       "example_com",
			URL:            "https://example.com/article",
			Title:          "Article Title",
			Body:           "Article content goes here with some text.",
			QualityScore:   50 + (i % 50), // Scores from 50-99
			IsCrimeRelated: isCrime,
			Topics:         topics,
			ContentType:    "article",
		}
	}

	// Route filter configuration
	route := Route{
		ID:              1,
		SourceID:        1,
		ChannelID:       1,
		MinQualityScore: 70,
		Topics:          []string{"crime"},
		IsActive:        true,
	}
	_ = route.ID
	_ = route.SourceID
	_ = route.ChannelID
	_ = route.IsActive

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		filtered := make([]Article, 0, 10)

		for i := range articles {
			article := &articles[i]
			// Check quality score threshold
			if article.QualityScore < route.MinQualityScore {
				continue
			}

			// Check topic match
			matchesTopic := false
			for _, routeTopic := range route.Topics {
				for _, articleTopic := range article.Topics {
					if articleTopic == routeTopic {
						matchesTopic = true
						break
					}
				}
				if matchesTopic {
					break
				}
			}

			if matchesTopic {
				filtered = append(filtered, *article)
			}
		}

		_ = filtered
	}
}

// BenchmarkRedisMessageSerialization benchmarks JSON serialization for Redis pub/sub
func BenchmarkRedisMessageSerialization(b *testing.B) {
	article := Article{
		SourceID:       "example_com",
		URL:            "https://example.com/article-123",
		CanonicalURL:   "https://example.com/article-123",
		Title:          "Breaking News: Major Event Downtown",
		Body:           "Full article content with multiple paragraphs of text describing the event in detail. This includes quotes from witnesses and official statements.",
		PublishedDate:  time.Now().UTC(),
		QualityScore:   85,
		IsCrimeRelated: true,
		Topics:         []string{"crime", "news", "local"},
		ContentType:    "article",
		Metadata: map[string]any{
			"author":  "Jane Reporter",
			"section": "crime",
		},
		ClassifiedAt: time.Now().UTC(),
	}

	// Publisher metadata
	message := map[string]any{
		"article": article,
		"metadata": map[string]any{
			"route_id":     1,
			"channel":      "articles:crime",
			"published_at": time.Now().UTC(),
		},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, err := json.Marshal(message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRouteProcessing benchmarks complete route processing pipeline
//
//nolint:gocognit // Benchmark function complexity is acceptable
func BenchmarkRouteProcessing(b *testing.B) {
	// 50 articles to process
	articles := make([]Article, 50)
	for i := range 50 {
		articles[i] = Article{
			SourceID:       "example_com",
			URL:            "https://example.com/article",
			Title:          "Article Title",
			Body:           "Article content",
			QualityScore:   60 + (i % 40),
			IsCrimeRelated: i%2 == 0,
			Topics:         []string{"news", "crime"},
			ContentType:    "article",
		}
	}

	// Multiple routes with different criteria
	routes := []Route{
		{ID: 1, MinQualityScore: 70, Topics: []string{"crime"}, IsActive: true},
		{ID: 2, MinQualityScore: 80, Topics: []string{"news"}, IsActive: true},
		{ID: 3, MinQualityScore: 60, Topics: []string{"local"}, IsActive: false}, // inactive
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, route := range routes {
			if !route.IsActive {
				continue
			}

			// Filter articles for this route
			for i := range articles {
				article := &articles[i]
				if article.QualityScore < route.MinQualityScore {
					continue
				}

				matchesTopic := false
				for _, routeTopic := range route.Topics {
					for _, articleTopic := range article.Topics {
						if articleTopic == routeTopic {
							matchesTopic = true
							break
						}
					}
					if matchesTopic {
						break
					}
				}

				if matchesTopic {
					// Prepare message
					message := map[string]any{
						"article": article,
						"metadata": map[string]any{
							"route_id": route.ID,
							"channel":  "articles:" + route.Topics[0],
						},
					}

					// Serialize
					//nolint:errchkjson // Benchmark test - intentionally ignoring error for performance
					_, _ = json.Marshal(message)
				}
			}
		}
	}
}

// BenchmarkTopicMatching benchmarks topic matching logic
func BenchmarkTopicMatching(b *testing.B) {
	routeTopics := []string{"crime", "local", "breaking"}
	articleTopics := []string{"news", "crime", "city"}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		matches := false
		for _, routeTopic := range routeTopics {
			for _, articleTopic := range articleTopics {
				if routeTopic == articleTopic {
					matches = true
					break
				}
			}
			if matches {
				break
			}
		}
		_ = matches
	}
}

// BenchmarkPublishHistoryCheck benchmarks deduplication check simulation
func BenchmarkPublishHistoryCheck(b *testing.B) {
	// Simulated publish history (article IDs already published)
	publishedArticles := make(map[string]bool, 1000)
	for i := range 1000 {
		publishedArticles["https://example.com/article/"+string(rune(i))] = true
	}

	testURLs := []string{
		"https://example.com/article/new1",
		"https://example.com/article/500", // exists
		"https://example.com/article/new2",
		"https://example.com/article/750", // exists
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, url := range testURLs {
			_ = publishedArticles[url] // Check if already published
		}
	}
}

// BenchmarkBatchProcessing benchmarks processing articles in batches
func BenchmarkBatchProcessing(b *testing.B) {
	const batchSize = 100

	// Create 1000 articles
	articles := make([]Article, 1000)
	for i := range 1000 {
		articles[i] = Article{
			URL:            "https://example.com/article",
			QualityScore:   70,
			IsCrimeRelated: true,
			Topics:         []string{"crime"},
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Process in batches
		for start := 0; start < len(articles); start += batchSize {
			end := start + batchSize
			if end > len(articles) {
				end = len(articles)
			}

			batch := articles[start:end]
			_ = batch
		}
	}
}

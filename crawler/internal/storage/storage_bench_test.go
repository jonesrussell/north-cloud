package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// BenchmarkJSONMarshaling benchmarks JSON marshaling of raw content for Elasticsearch
func BenchmarkJSONMarshaling(b *testing.B) {
	// Create realistic raw content document
	document := map[string]interface{}{
		"source_id":      "example_com",
		"url":            "https://example.com/article-12345",
		"canonical_url":  "https://example.com/article-12345",
		"title":          "Breaking News: Major Event Unfolds in City Center",
		"raw_text":       "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.",
		"raw_html":       "<html><head><title>Breaking News</title></head><body><article><h1>Breaking News: Major Event Unfolds in City Center</h1><p>Lorem ipsum dolor sit amet...</p></article></body></html>",
		"published_date": time.Now().UTC().Format(time.RFC3339),
		"og_tags": map[string]string{
			"og:title":       "Breaking News Article",
			"og:description": "Full coverage of today's major event",
			"og:image":       "https://example.com/images/article.jpg",
			"og:type":        "article",
		},
		"metadata": map[string]interface{}{
			"author":     "Jane Reporter",
			"section":    "news",
			"word_count": 250,
		},
		"classification_status": "pending",
		"discovered_at":         time.Now().UTC().Format(time.RFC3339),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(document)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIndexDocumentPreparation benchmarks preparing documents for bulk indexing
func BenchmarkIndexDocumentPreparation(b *testing.B) {
	documents := make([]map[string]interface{}, 100)

	for i := 0; i < 100; i++ {
		documents[i] = map[string]interface{}{
			"source_id":             "example_com",
			"url":                   "https://example.com/article",
			"title":                 "Article Title",
			"raw_text":              "Article content text",
			"classification_status": "pending",
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate bulk indexing preparation
		bulkBody := make([]map[string]interface{}, 0, len(documents)*2)

		for _, doc := range documents {
			// Add index metadata
			metadata := map[string]interface{}{
				"index": map[string]interface{}{
					"_index": "example_com_raw_content",
					"_id":    doc["url"],
				},
			}
			bulkBody = append(bulkBody, metadata)

			// Add document
			bulkBody = append(bulkBody, doc)
		}
	}
}

// BenchmarkIndexNameGeneration benchmarks generating index names from source IDs
func BenchmarkIndexNameGeneration(b *testing.B) {
	sourceIDs := []string{
		"example.com",
		"news-site.org",
		"the-daily-times.com",
		"local-news-network.ca",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, sourceID := range sourceIDs {
			// Simulate index name generation logic
			indexName := generateIndexName(sourceID, "raw_content")
			_ = indexName
		}
	}
}

// BenchmarkQueryBuilding benchmarks building Elasticsearch queries
func BenchmarkQueryBuilding(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Build query for pending classification
		query := map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []map[string]interface{}{
						{
							"term": map[string]interface{}{
								"classification_status": "pending",
							},
						},
					},
				},
			},
			"sort": []map[string]interface{}{
				{
					"discovered_at": map[string]string{
						"order": "asc",
					},
				},
			},
			"size": 100,
		}

		_, err := json.Marshal(query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkURLNormalization benchmarks URL normalization for document IDs
func BenchmarkURLNormalization(b *testing.B) {
	urls := []string{
		"https://example.com/article?utm_source=twitter&utm_medium=social",
		"https://www.example.com/news/breaking-story",
		"http://example.com/article#comments",
		"https://example.com/path/to/article/",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			// Simulate URL normalization
			normalized := normalizeURL(url)
			_ = normalized
		}
	}
}

// Helper functions for benchmarks

func generateIndexName(sourceID, suffix string) string {
	// Simple implementation for benchmarking
	return sourceID + "_" + suffix
}

func normalizeURL(url string) string {
	// Simplified normalization for benchmarking
	// In reality, this would parse and clean the URL
	return url
}

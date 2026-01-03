package classifier

import (
	"strings"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// BenchmarkCrimeDetection benchmarks crime classification logic
func BenchmarkCrimeDetection(b *testing.B) {
	testCases := []string{
		"Police arrested three suspects in connection with a robbery downtown last night.",
		"The defendant was charged with assault and battery after the incident.",
		"A shooting occurred near the intersection of Main and 5th Street.",
		"Breaking and entering reported at local business, investigation ongoing.",
		"Traffic violations increased 20% this quarter according to city data.",
	}

	// Crime keywords typically used for detection
	crimeKeywords := []string{
		"arrest", "arrested", "police", "crime", "criminal",
		"theft", "robbery", "burglary", "assault", "murder",
		"shooting", "investigation", "suspect", "charged",
		"breaking and entering", "violation",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, text := range testCases {
			lowerText := strings.ToLower(text)
			isCrime := false

			for _, keyword := range crimeKeywords {
				if strings.Contains(lowerText, keyword) {
					isCrime = true
					break
				}
			}

			_ = isCrime
		}
	}
}

// BenchmarkQualityScoring benchmarks quality score calculation
func BenchmarkQualityScoring(b *testing.B) {
	rawContent := &domain.RawContent{
		SourceID:      "example_com",
		URL:           "https://example.com/article",
		CanonicalURL:  "https://example.com/article",
		Title:         "Breaking News: Major Event Unfolds Downtown",
		RawText:       strings.Repeat("This is article content with meaningful text. ", 50), // ~400 words
		RawHTML:       "<html><body><article><h1>Title</h1><p>Content</p></article></body></html>",
		PublishedDate: time.Now().UTC(),
		OGTags: map[string]string{
			"og:title":       "Breaking News Article",
			"og:description": "Detailed coverage of today's event",
			"og:image":       "https://example.com/image.jpg",
			"og:type":        "article",
		},
		Metadata: map[string]interface{}{
			"author":  "Jane Reporter",
			"section": "news",
		},
		ClassificationStatus: "pending",
		DiscoveredAt:        time.Now().UTC(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		score := 0

		// Title present (20 points)
		if rawContent.Title != "" && len(rawContent.Title) > 10 {
			score += 20
		}

		// Adequate text content (30 points)
		wordCount := len(strings.Fields(rawContent.RawText))
		if wordCount >= 300 {
			score += 30
		} else if wordCount >= 100 {
			score += 15
		}

		// Published date (10 points)
		if !rawContent.PublishedDate.IsZero() {
			score += 10
		}

		// OG tags present (20 points)
		if len(rawContent.OGTags) >= 3 {
			score += 20
		}

		// Canonical URL (10 points)
		if rawContent.CanonicalURL != "" {
			score += 10
		}

		// Metadata present (10 points)
		if len(rawContent.Metadata) > 0 {
			score += 10
		}

		_ = score // Quality score 0-100
	}
}

// BenchmarkContentTypeDetection benchmarks content type classification
func BenchmarkContentTypeDetection(b *testing.B) {
	testContent := []struct {
		ogType   string
		title    string
		url      string
		expected string
	}{
		{"article", "News Article Title", "https://example.com/news/article", "article"},
		{"video", "Watch: Video Title", "https://example.com/videos/123", "video"},
		{"", "Jobs: Software Engineer", "https://example.com/jobs/engineer", "job"},
		{"website", "About Us - Company", "https://example.com/about", "page"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tc := range testContent {
			contentType := "article" // default

			// Check OG type
			if tc.ogType != "" {
				contentType = tc.ogType
			} else {
				// Heuristics-based detection
				lowerTitle := strings.ToLower(tc.title)
				lowerURL := strings.ToLower(tc.url)

				if strings.Contains(lowerTitle, "video") || strings.Contains(lowerTitle, "watch") {
					contentType = "video"
				} else if strings.Contains(lowerTitle, "jobs") || strings.Contains(lowerURL, "/jobs/") {
					contentType = "job"
				} else if strings.Contains(lowerURL, "/about") || strings.Contains(lowerURL, "/contact") {
					contentType = "page"
				}
			}

			_ = contentType
		}
	}
}

// BenchmarkTopicClassification benchmarks topic/category detection
func BenchmarkTopicClassification(b *testing.B) {
	articles := []string{
		"Police arrested three suspects in a downtown robbery last night near Main Street.",
		"The local sports team won the championship game in overtime thriller.",
		"New technology startup raises $50M in Series A funding round.",
		"City council approves budget for next fiscal year after heated debate.",
		"Health officials warn of flu outbreak, recommend vaccinations.",
	}

	topicKeywords := map[string][]string{
		"crime":      {"police", "arrest", "robbery", "crime", "suspect"},
		"sports":     {"team", "game", "championship", "win", "player"},
		"business":   {"startup", "funding", "company", "market", "investment"},
		"politics":   {"council", "government", "vote", "election", "policy"},
		"health":     {"health", "medical", "doctor", "disease", "treatment"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, article := range articles {
			lowerArticle := strings.ToLower(article)
			topics := make([]string, 0, 2)

			for topic, keywords := range topicKeywords {
				matches := 0
				for _, keyword := range keywords {
					if strings.Contains(lowerArticle, keyword) {
						matches++
					}
				}

				if matches >= 2 {
					topics = append(topics, topic)
				}
			}

			_ = topics
		}
	}
}

// BenchmarkFullClassification benchmarks the complete classification pipeline
func BenchmarkFullClassification(b *testing.B) {
	rawContent := &domain.RawContent{
		SourceID:      "example_com",
		URL:           "https://example.com/news/crime-report",
		CanonicalURL:  "https://example.com/news/crime-report",
		Title:         "Police Arrest Suspect in Downtown Robbery Investigation",
		RawText:       "Police arrested a suspect early this morning in connection with a series of robberies that occurred downtown over the past month. The investigation led authorities to identify and apprehend the individual after reviewing security footage and witness statements.",
		RawHTML:       "<html><body><article><h1>Police Arrest Suspect</h1><p>Content here...</p></article></body></html>",
		PublishedDate: time.Now().UTC(),
		OGTags: map[string]string{
			"og:title":       "Police Arrest Suspect in Robbery",
			"og:description": "Authorities make arrest after month-long investigation",
			"og:image":       "https://example.com/images/news.jpg",
			"og:type":        "article",
		},
		Metadata: map[string]interface{}{
			"author":  "Crime Reporter",
			"section": "crime",
		},
		ClassificationStatus: "pending",
		DiscoveredAt:        time.Now().UTC(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Step 1: Content type detection
		contentType := "article"
		if ogType, ok := rawContent.OGTags["og:type"]; ok {
			contentType = ogType
		}

		// Step 2: Quality scoring
		qualityScore := 0
		if rawContent.Title != "" {
			qualityScore += 20
		}
		wordCount := len(strings.Fields(rawContent.RawText))
		if wordCount >= 100 {
			qualityScore += 30
		}
		if len(rawContent.OGTags) >= 3 {
			qualityScore += 20
		}
		qualityScore += 30 // Other factors

		// Step 3: Crime detection
		lowerText := strings.ToLower(rawContent.RawText + " " + rawContent.Title)
		isCrime := strings.Contains(lowerText, "police") ||
			strings.Contains(lowerText, "arrest") ||
			strings.Contains(lowerText, "crime") ||
			strings.Contains(lowerText, "robbery")

		// Step 4: Topic classification
		topics := []string{}
		if isCrime {
			topics = append(topics, "crime")
		}

		// Create classification result
		_ = struct {
			ContentType  string
			QualityScore int
			IsCrime      bool
			Topics       []string
		}{
			ContentType:  contentType,
			QualityScore: qualityScore,
			IsCrime:      isCrime,
			Topics:       topics,
		}
	}
}

// BenchmarkSourceReputationCalculation benchmarks source reputation scoring
func BenchmarkSourceReputationCalculation(b *testing.B) {
	// Simulated historical quality scores for a source
	qualityScores := []int{75, 80, 72, 85, 78, 82, 79, 81, 76, 83}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Calculate average quality score
		sum := 0
		for _, score := range qualityScores {
			sum += score
		}
		avgScore := sum / len(qualityScores)

		// Calculate reputation (0-100 scale)
		reputation := avgScore

		// Adjust for consistency (standard deviation penalty)
		variance := 0
		for _, score := range qualityScores {
			diff := score - avgScore
			variance += diff * diff
		}
		variance /= len(qualityScores)

		// Penalize high variance
		if variance > 100 {
			reputation -= 5
		} else if variance > 50 {
			reputation -= 2
		}

		_ = reputation
	}
}

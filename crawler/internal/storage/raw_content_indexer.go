package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/storage/types"
)

// RawContent represents minimally-processed content for classification
// This matches the classifier's domain.RawContent structure
type RawContent struct {
	ID                   string     `json:"id"`
	URL                  string     `json:"url"`
	SourceName           string     `json:"source_name"`
	Title                string     `json:"title"`
	RawText              string     `json:"raw_text"`
	RawHTML              string     `json:"raw_html,omitempty"`
	MetaDescription      string     `json:"meta_description,omitempty"`
	MetaKeywords         string     `json:"meta_keywords,omitempty"`
	OGType               string     `json:"og_type,omitempty"`
	OGTitle              string     `json:"og_title,omitempty"`
	OGDescription        string     `json:"og_description,omitempty"`
	OGImage              string     `json:"og_image,omitempty"`
	Author               string     `json:"author,omitempty"`
	PublishedDate        *time.Time `json:"published_date,omitempty"`
	ClassificationStatus string     `json:"classification_status"`
	CrawledAt            time.Time  `json:"crawled_at"`
	WordCount            int        `json:"word_count"`
}

// RawContentIndexer handles indexing of raw content for the classifier
type RawContentIndexer struct {
	storage types.Interface
	logger  logger.Interface
}

// NewRawContentIndexer creates a new raw content indexer
func NewRawContentIndexer(storage types.Interface, logger logger.Interface) *RawContentIndexer {
	return &RawContentIndexer{
		storage: storage,
		logger:  logger,
	}
}

// IndexArticle indexes an article as raw content for classification
func (r *RawContentIndexer) IndexArticle(ctx context.Context, article *domain.Article, sourceName string) error {
	if article == nil {
		return fmt.Errorf("article is nil")
	}

	// Convert article to raw content
	rawContent := r.convertArticleToRawContent(article, sourceName)

	// Index to raw_content index
	indexName := r.getRawContentIndexName(sourceName)

	r.logger.Debug("Indexing raw content for classification",
		"index", indexName,
		"article_id", article.ID,
		"source_name", sourceName,
		"word_count", rawContent.WordCount,
	)

	err := r.storage.IndexDocument(ctx, indexName, rawContent.ID, rawContent)
	if err != nil {
		r.logger.Error("Failed to index raw content",
			"error", err,
			"index", indexName,
			"article_id", article.ID,
		)
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	r.logger.Info("Indexed raw content for classification",
		"index", indexName,
		"article_id", article.ID,
		"classification_status", rawContent.ClassificationStatus,
	)

	return nil
}

// convertArticleToRawContent converts a crawler Article to RawContent for classification
func (r *RawContentIndexer) convertArticleToRawContent(article *domain.Article, sourceName string) *RawContent {
	// Combine keywords from meta tags
	var metaKeywords string
	if len(article.Keywords) > 0 {
		metaKeywords = strings.Join(article.Keywords, ", ")
	}

	// Determine OG type (article vs page)
	ogType := "article" // Default to article for news content

	// Use published date if available
	var publishedDate *time.Time
	if !article.PublishedDate.IsZero() {
		publishedDate = &article.PublishedDate
	}

	rawContent := &RawContent{
		ID:                   article.ID,
		URL:                  article.Source,
		SourceName:           sourceName,
		Title:                article.Title,
		RawText:              article.Body,
		MetaDescription:      article.Description,
		MetaKeywords:         metaKeywords,
		OGType:               ogType,
		OGTitle:              article.OgTitle,
		OGDescription:        article.OgDescription,
		OGImage:              article.OgImage,
		Author:               article.Author,
		PublishedDate:        publishedDate,
		ClassificationStatus: "pending",
		CrawledAt:            time.Now(),
		WordCount:            article.WordCount,
	}

	return rawContent
}

// getRawContentIndexName returns the index name for raw content
// Format: {source}_raw_content
// Example: example_com_raw_content
func (r *RawContentIndexer) getRawContentIndexName(sourceName string) string {
	// Normalize source name for index naming
	// Replace dots with underscores, remove protocol, etc.
	normalized := strings.ReplaceAll(sourceName, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ToLower(normalized)

	return fmt.Sprintf("%s_raw_content", normalized)
}

// EnsureRawContentIndex ensures the raw_content index exists with proper mappings
func (r *RawContentIndexer) EnsureRawContentIndex(ctx context.Context, sourceName string) error {
	indexName := r.getRawContentIndexName(sourceName)

	// Define raw content index mapping
	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":                    map[string]string{"type": "keyword"},
				"url":                   map[string]string{"type": "keyword"},
				"source_name":           map[string]string{"type": "keyword"},
				"title":                 map[string]string{"type": "text"},
				"raw_text":              map[string]string{"type": "text"},
				"raw_html":              map[string]interface{}{"type": "text", "index": "false"}, // Store but don't index
				"meta_description":      map[string]string{"type": "text"},
				"meta_keywords":         map[string]string{"type": "text"},
				"og_type":               map[string]string{"type": "keyword"},
				"og_title":              map[string]string{"type": "text"},
				"og_description":        map[string]string{"type": "text"},
				"og_image":              map[string]string{"type": "keyword"},
				"author":                map[string]string{"type": "text"},
				"published_date":        map[string]string{"type": "date"},
				"classification_status": map[string]string{"type": "keyword"},
				"crawled_at":            map[string]string{"type": "date"},
				"word_count":            map[string]string{"type": "integer"},
			},
		},
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
	}

	// Convert mapping to JSON
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal index mapping: %w", err)
	}

	r.logger.Info("Creating raw_content index",
		"index", indexName,
		"source_name", sourceName,
	)

	// Create index using storage
	indexManager := r.storage.GetIndexManager()
	err = indexManager.EnsureIndex(ctx, indexName, string(mappingJSON))
	if err != nil {
		return fmt.Errorf("failed to ensure raw_content index: %w", err)
	}

	r.logger.Info("Created raw_content index",
		"index", indexName,
	)

	return nil
}

// GetPendingCount returns the count of pending items in raw_content index
func (r *RawContentIndexer) GetPendingCount(ctx context.Context, sourceName string) (int, error) {
	indexName := r.getRawContentIndexName(sourceName)

	// Execute search to get count
	// Note: This requires extending the storage interface or using search manager
	// For now, return 0 as placeholder
	r.logger.Debug("Getting pending count",
		"index", indexName,
		"note", "Count query not fully implemented - placeholder returns 0",
	)

	return 0, nil
}

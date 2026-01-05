package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// RawContent represents minimally-processed content for classification
// This matches the classifier's domain.RawContent structure
type RawContent struct {
	ID                   string         `json:"id"`
	URL                  string         `json:"url"`
	SourceName           string         `json:"source_name"`
	Title                string         `json:"title"`
	RawText              string         `json:"raw_text"`
	RawHTML              string         `json:"raw_html,omitempty"` // Large field, omit if empty
	MetaDescription      string         `json:"meta_description"`   // Classifier needs this
	MetaKeywords         string         `json:"meta_keywords,omitempty"`
	OGType               string         `json:"og_type"`        // CRITICAL: Classifier needs this
	OGTitle              string         `json:"og_title"`       // Classifier needs this
	OGDescription        string         `json:"og_description"` // Classifier needs this
	OGImage              string         `json:"og_image,omitempty"`
	Author               string         `json:"author,omitempty"`
	PublishedDate        *time.Time     `json:"published_date"` // CRITICAL: Classifier needs this
	CanonicalURL         string         `json:"canonical_url,omitempty"`
	ArticleSection       string         `json:"article_section,omitempty"`
	JSONLDData           map[string]any `json:"json_ld_data,omitempty"`
	ClassificationStatus string         `json:"classification_status"`
	CrawledAt            time.Time      `json:"crawled_at"`
	WordCount            int            `json:"word_count"` // CRITICAL: Classifier needs this
	Meta                 map[string]any `json:"meta,omitempty"` // Additional metadata
}

// RawContentIndexer handles indexing of raw content for the classifier
type RawContentIndexer struct {
	storage        types.Interface
	logger         logger.Interface
	ensuredIndexes sync.Map // Cache of indexes that have been ensured (map[string]bool)
}

// NewRawContentIndexer creates a new raw content indexer
func NewRawContentIndexer(storage types.Interface, log logger.Interface) *RawContentIndexer {
	return &RawContentIndexer{
		storage: storage,
		logger:  log,
	}
}

// IndexRawContent indexes raw content for classification
func (r *RawContentIndexer) IndexRawContent(ctx context.Context, rawContent *RawContent) error {
	if rawContent == nil {
		return errors.New("raw content is nil")
	}

	// Index to raw_content index
	indexName := r.getRawContentIndexName(rawContent.SourceName)

	r.logger.Debug("Indexing raw content for classification",
		"index", indexName,
		"content_id", rawContent.ID,
		"source_name", rawContent.SourceName,
		"word_count", rawContent.WordCount,
	)

	err := r.storage.IndexDocument(ctx, indexName, rawContent.ID, rawContent)
	if err != nil {
		r.logger.Error("Failed to index raw content",
			"error", err,
			"index", indexName,
			"content_id", rawContent.ID,
		)
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	r.logger.Info("Indexed raw content for classification",
		"index", indexName,
		"content_id", rawContent.ID,
		"classification_status", rawContent.ClassificationStatus,
	)

	return nil
}

var (
	// invalidIndexNameChars matches all characters that are invalid in Elasticsearch index names.
	// Invalid characters: space, ", *, ,, /, <, >, ?, \, |
	invalidIndexNameChars = regexp.MustCompile(`[\s"*,/<>?\\|]`)
	// consecutiveUnderscores matches two or more consecutive underscores.
	consecutiveUnderscores = regexp.MustCompile(`_{2,}`)
)

// sanitizeIndexName sanitizes a source name for use in Elasticsearch index names.
// Elasticsearch index names cannot contain: space, ", *, ,, /, <, >, ?, \, |
// This function replaces invalid characters with underscores, normalizes dots/dashes,
// removes leading/trailing underscores, and collapses consecutive underscores.
func sanitizeIndexName(sourceName string) string {
	if sourceName == "" {
		return "unknown"
	}

	// Convert to lowercase first
	normalized := strings.ToLower(sourceName)

	// Replace invalid Elasticsearch index name characters with underscores in one pass
	normalized = invalidIndexNameChars.ReplaceAllString(normalized, "_")

	// Replace dots and dashes with underscores (existing behavior)
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	// Collapse consecutive underscores into a single underscore
	normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")

	// Remove leading and trailing underscores
	normalized = strings.Trim(normalized, "_")

	// Handle edge case: if all characters were invalid, return fallback
	if normalized == "" {
		return "unknown"
	}

	return normalized
}

// getRawContentIndexName returns the index name for raw content
// Format: {source}_raw_content
// Example: example_com_raw_content
func (r *RawContentIndexer) getRawContentIndexName(sourceName string) string {
	normalized := sanitizeIndexName(sourceName)
	return fmt.Sprintf("%s_raw_content", normalized)
}

// EnsureRawContentIndex ensures the raw_content index exists with proper mappings.
// Uses a cache to avoid redundant checks and log messages for indexes that have already been ensured.
func (r *RawContentIndexer) EnsureRawContentIndex(ctx context.Context, sourceName string) error {
	indexName := r.getRawContentIndexName(sourceName)

	// Check cache first - if we've already ensured this index, skip the check
	if _, alreadyEnsured := r.ensuredIndexes.Load(indexName); alreadyEnsured {
		return nil
	}

	// Define raw content index mapping
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"id":                    map[string]string{"type": "keyword"},
				"url":                   map[string]string{"type": "keyword"},
				"source_name":           map[string]string{"type": "keyword"},
				"title":                 map[string]string{"type": "text"},
				"raw_text":              map[string]string{"type": "text"},
				"raw_html":              map[string]any{"type": "text", "index": "false"}, // Store but don't index
				"meta_description":      map[string]string{"type": "text"},
				"meta_keywords":         map[string]string{"type": "text"},
				"og_type":               map[string]string{"type": "keyword"},
				"og_title":              map[string]string{"type": "text"},
				"og_description":        map[string]string{"type": "text"},
				"og_image":              map[string]string{"type": "keyword"},
				"author":                map[string]string{"type": "text"},
				"published_date":        map[string]string{"type": "date"},
				"canonical_url":         map[string]string{"type": "keyword"},
				"article_section":       map[string]string{"type": "keyword"},
				"json_ld_data":          map[string]string{"type": "object"},
				"classification_status": map[string]string{"type": "keyword"},
				"crawled_at":            map[string]string{"type": "date"},
				"word_count":            map[string]string{"type": "integer"},
				"meta":                  map[string]string{"type": "object"},
			},
		},
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
	}

	// Convert mapping to JSON
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal index mapping: %w", err)
	}

	r.logger.Info("Ensuring raw_content index",
		"index", indexName,
		"source_name", sourceName,
	)

	// Create index using storage
	indexManager := r.storage.GetIndexManager()
	err = indexManager.EnsureIndex(ctx, indexName, string(mappingJSON))
	if err != nil {
		return fmt.Errorf("failed to ensure raw_content index: %w", err)
	}

	// Cache that this index has been ensured
	r.ensuredIndexes.Store(indexName, true)

	return nil
}

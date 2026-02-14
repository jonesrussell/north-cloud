package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
	infralogger "github.com/north-cloud/infrastructure/logger"
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
	WordCount            int            `json:"word_count"`     // CRITICAL: Classifier needs this
	Meta                 map[string]any `json:"meta,omitempty"` // Additional metadata
}

// RawContentIndexer handles indexing of raw content for the classifier
type RawContentIndexer struct {
	storage        types.Interface
	logger         infralogger.Logger
	ensuredIndexes sync.Map // Cache of indexes that have been ensured (map[string]bool)
}

// NewRawContentIndexer creates a new raw content indexer
func NewRawContentIndexer(storage types.Interface, log infralogger.Logger) *RawContentIndexer {
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
		infralogger.String("index", indexName),
		infralogger.String("content_id", rawContent.ID),
		infralogger.String("source_name", rawContent.SourceName),
		infralogger.Int("word_count", rawContent.WordCount),
	)

	err := r.storage.IndexDocument(ctx, indexName, rawContent.ID, rawContent)
	if err != nil {
		r.logger.Error("Failed to index raw content",
			infralogger.Error(err),
			infralogger.String("index", indexName),
			infralogger.String("content_id", rawContent.ID),
		)
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	r.logger.Info("Indexed raw content for classification",
		infralogger.String("index", indexName),
		infralogger.String("content_id", rawContent.ID),
		infralogger.String("classification_status", rawContent.ClassificationStatus),
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

// EnsureRawContentIndex ensures the raw_content index exists.
// The canonical mapping is managed by the index-manager service.
// Uses a cache to avoid redundant checks for indexes that have already been ensured.
func (r *RawContentIndexer) EnsureRawContentIndex(ctx context.Context, sourceName string) error {
	indexName := r.getRawContentIndexName(sourceName)

	if _, alreadyEnsured := r.ensuredIndexes.Load(indexName); alreadyEnsured {
		return nil
	}

	r.logger.Info("Ensuring raw_content index",
		infralogger.String("index", indexName),
		infralogger.String("source_name", sourceName),
	)

	indexManager := r.storage.GetIndexManager()
	mapping := contracts.RawContentIndexMapping()
	err := indexManager.EnsureIndex(ctx, indexName, mapping)
	if err != nil {
		return fmt.Errorf("failed to ensure raw_content index: %w", err)
	}

	r.ensuredIndexes.Store(indexName, true)
	return nil
}

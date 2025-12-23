# Crawler Integration for Classifier Microservice

## Overview

This document describes how the crawler integrates with the classifier microservice through dual indexing. The crawler now indexes content to both its original indexes (for immediate use) and to a `raw_content` index (for classification processing).

## Architecture

```
┌─────────────┐
│   Crawler   │
└──────┬──────┘
       │
       ├──────────────────┬───────────────────┐
       │                  │                   │
       ▼                  ▼                   ▼
┌──────────────┐  ┌────────────────┐  ┌──────────────────┐
│   Articles   │  │  Raw Content   │  │   Pages (opt)    │
│    Index     │  │     Index      │  │     Index        │
│ (existing)   │  │    (new)       │  │   (existing)     │
└──────────────┘  └────────┬───────┘  └──────────────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │   Classifier    │
                  │   Microservice  │
                  └────────┬────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │   Classified    │
                  │    Content      │
                  └─────────────────┘
```

## Raw Content Indexer

### Location
`/crawler/internal/storage/raw_content_indexer.go`

### Purpose
Indexes minimally-processed content from the crawler to Elasticsearch for the classifier to process.

### Key Features

1. **Minimal Processing**: Stores raw text without classification
2. **Metadata Preservation**: Keeps OG tags, metadata, and structure
3. **Status Tracking**: Uses `classification_status` field (pending/classified/failed)
4. **Source-Based Indexing**: Creates separate indexes per source

### RawContent Structure

Matches the classifier's `domain.RawContent` model:

```go
type RawContent struct {
    ID                   string     `json:"id"`
    URL                  string     `json:"url"`
    SourceName           string     `json:"source_name"`
    Title                string     `json:"title"`
    RawText              string     `json:"raw_text"`
    RawHTML              string     `json:"raw_html,omitempty"`      // Not indexed
    MetaDescription      string     `json:"meta_description,omitempty"`
    MetaKeywords         string     `json:"meta_keywords,omitempty"`
    OGType               string     `json:"og_type,omitempty"`
    OGTitle              string     `json:"og_title,omitempty"`
    OGDescription        string     `json:"og_description,omitempty"`
    OGImage              string     `json:"og_image,omitempty"`
    Author               string     `json:"author,omitempty"`
    PublishedDate        *time.Time `json:"published_date,omitempty"`
    ClassificationStatus string     `json:"classification_status"`   // pending/classified/failed
    CrawledAt            time.Time  `json:"crawled_at"`
    WordCount            int        `json:"word_count"`
}
```

### Index Naming Convention

Raw content indexes follow the pattern: `{source}_raw_content`

**Examples**:
- `example_com_raw_content` (for example.com)
- `news_site_org_raw_content` (for news-site.org)

**Normalization Rules**:
- Dots (.) → underscores (_)
- Hyphens (-) → underscores (_)
- Lowercase only

### Index Mapping

```json
{
  "mappings": {
    "properties": {
      "id":                    { "type": "keyword" },
      "url":                   { "type": "keyword" },
      "source_name":           { "type": "keyword" },
      "title":                 { "type": "text" },
      "raw_text":              { "type": "text" },
      "raw_html":              { "type": "text", "index": false },
      "meta_description":      { "type": "text" },
      "meta_keywords":         { "type": "text" },
      "og_type":               { "type": "keyword" },
      "og_title":              { "type": "text" },
      "og_description":        { "type": "text" },
      "og_image":              { "type": "keyword" },
      "author":                { "type": "text" },
      "published_date":        { "type": "date" },
      "classification_status": { "type": "keyword" },
      "crawled_at":            { "type": "date" },
      "word_count":            { "type": "integer" }
    }
  },
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 1
  }
}
```

## Usage

### 1. Create Raw Content Indexer

```go
import (
    "github.com/jonesrussell/gocrawl/internal/storage"
)

// Create indexer
rawIndexer := storage.NewRawContentIndexer(storageInstance, logger)

// Ensure index exists for a source
err := rawIndexer.EnsureRawContentIndex(ctx, "example.com")
if err != nil {
    log.Fatal(err)
}
```

### 2. Index Article as Raw Content

```go
// After crawling an article
article := &domain.Article{
    ID:            "article-123",
    Title:         "News Article Title",
    Body:          "Article content...",
    Source:        "https://example.com/article",
    WordCount:     500,
    PublishedDate: time.Now(),
    // ... other fields
}

// Index to raw_content for classification
err := rawIndexer.IndexArticle(ctx, article, "example.com")
if err != nil {
    logger.Error("Failed to index raw content", "error", err)
}
```

### 3. Integration in Crawler Processor

The crawler should be updated to perform dual indexing:

```go
func (p *ArticleProcessor) Process(ctx context.Context, article *domain.Article) error {
    // 1. Index to regular article index (existing behavior)
    err := p.storage.IndexDocument(ctx, p.articleIndex, article.ID, article)
    if err != nil {
        return fmt.Errorf("failed to index article: %w", err)
    }

    // 2. Index to raw_content for classification (new behavior)
    err = p.rawIndexer.IndexArticle(ctx, article, p.sourceName)
    if err != nil {
        // Log error but don't fail - classification is optional
        p.logger.Warn("Failed to index raw content for classification",
            "article_id", article.ID,
            "error", err,
        )
    }

    return nil
}
```

## Classifier Integration

### Polling for Pending Content

The classifier's poller queries for pending content:

```go
// Query for pending items
query := map[string]interface{}{
    "query": map[string]interface{}{
        "term": map[string]interface{}{
            "classification_status": "pending",
        },
    },
    "size": 100,
}
```

### Processing Flow

1. **Crawler** indexes article to `{source}_raw_content` with `status=pending`
2. **Classifier Poller** queries for `status=pending` every 30 seconds
3. **Batch Processor** classifies content (quality, topics, reputation)
4. **Classifier** indexes results to `classified_content`
5. **Classifier** updates `raw_content` status to `classified`

### Status Transitions

```
pending → processing → classified
                    ↘ failed
```

## Configuration

### Crawler Configuration

Add to crawler config:

```yaml
storage:
  enable_raw_content_indexing: true  # Enable dual indexing
  raw_content_batch_size: 50        # Batch size for raw content indexing
```

### Environment Variables

```bash
# Enable raw content indexing
ENABLE_RAW_CONTENT_INDEXING=true

# Classifier service URL (for future direct API integration)
CLASSIFIER_URL=http://localhost:8070
```

## Performance Considerations

### Indexing Overhead

- **Additional ES calls**: +1 index operation per article
- **Network**: Minimal (same ES cluster)
- **Storage**: ~2x storage (raw + classified content)

### Optimization Strategies

1. **Batch Indexing**: Index raw content in batches
2. **Async Processing**: Use goroutines for parallel indexing
3. **Conditional Indexing**: Only index if classification is enabled
4. **Error Handling**: Don't fail crawl if raw indexing fails

### Example Performance

For 1000 articles:
- Regular indexing: 1000 ES calls, ~10s
- Dual indexing: 2000 ES calls, ~15s (+50% overhead)
- With batching: ~12s (+20% overhead)

## Monitoring

### Metrics to Track

1. **Raw Content Indexed**: Count of items indexed to raw_content
2. **Indexing Failures**: Count of failed raw content indexes
3. **Pending Items**: Count of items awaiting classification
4. **Processing Time**: Time to index raw content

### Logging

```go
logger.Info("Indexed raw content for classification",
    "index", indexName,
    "article_id", article.ID,
    "classification_status", "pending",
    "word_count", article.WordCount,
)
```

## Error Handling

### Non-Blocking Failures

Raw content indexing failures should **NOT** block the crawler:

```go
// Index to raw_content (non-blocking)
if err := rawIndexer.IndexArticle(ctx, article, sourceName); err != nil {
    logger.Warn("Failed to index raw content - continuing",
        "article_id", article.ID,
        "error", err,
    )
    // Don't return error - crawler continues
}
```

### Retry Strategy

For transient failures:
1. Log warning
2. Continue crawling
3. Classifier will pick up missed items later if needed

## Future Enhancements

### 1. Direct API Integration

Instead of indexing to ES, crawler could POST directly to classifier API:

```go
// POST /api/v1/classify
resp, err := http.Post(
    classifierURL+"/api/v1/classify",
    "application/json",
    articleJSON,
)
```

**Pros**:
- Immediate classification
- No polling required
- Simpler architecture

**Cons**:
- Tight coupling
- Synchronous (slower crawling)
- Requires classifier to be always available

### 2. Queue-Based Processing

Use RabbitMQ or Kafka:

```
Crawler → Queue → Classifier
```

**Pros**:
- Decoupled
- Buffering
- Scalable

**Cons**:
- Additional infrastructure
- More complex

### 3. Selective Indexing

Only index certain content types:

```go
if article.WordCount > 100 && article.PublishedDate.After(cutoff) {
    rawIndexer.IndexArticle(ctx, article, sourceName)
}
```

## Testing

### Unit Tests

```go
func TestRawContentIndexer_IndexArticle(t *testing.T) {
    // Create mock storage
    mockStorage := &MockStorage{}
    logger := &MockLogger{}
    indexer := storage.NewRawContentIndexer(mockStorage, logger)

    // Create test article
    article := &domain.Article{
        ID:        "test-123",
        Title:     "Test Article",
        Body:      "Test content",
        WordCount: 100,
    }

    // Index article
    err := indexer.IndexArticle(context.Background(), article, "example.com")
    if err != nil {
        t.Fatalf("IndexArticle failed: %v", err)
    }

    // Verify indexed
    if len(mockStorage.IndexedDocs) != 1 {
        t.Errorf("expected 1 document indexed, got %d", len(mockStorage.IndexedDocs))
    }
}
```

### Integration Tests

Test end-to-end flow:
1. Crawler indexes to raw_content
2. Classifier polls and finds item
3. Classifier processes and updates status
4. Verify status updated to "classified"

## Troubleshooting

### Issue: Raw content not being indexed

**Check**:
1. Is raw content indexing enabled in config?
2. Does the index exist? Check ES: `GET /{source}_raw_content`
3. Are there errors in logs?

### Issue: Classifier not finding pending items

**Check**:
1. Verify index name matches pattern: `{source}_raw_content`
2. Check classification_status field: should be "pending"
3. Query ES directly: `GET /{source}_raw_content/_search?q=classification_status:pending`

### Issue: Duplicate indexing

**Check**:
1. Ensure article IDs are unique
2. Verify crawler isn't re-crawling same content
3. Check ES `_id` field

## References

- [Crawler README](/crawler/README.md)
- [Classifier Week 3 Summary](/classifier/docs/WEEK_3_SUMMARY.md)
- [Classifier Week 4 Summary](/classifier/docs/WEEK_4_SUMMARY.md)
- [CLAUDE.md - Main Architecture](/CLAUDE.md)

---

**Status**: ✅ Raw Content Indexer Complete
**Date**: 2025-12-22
**Integration**: Ready for crawler processor updates

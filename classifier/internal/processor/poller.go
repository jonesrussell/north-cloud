package processor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	// maxURLLength is the maximum reasonable URL length (2048 chars is common browser limit)
	// URLs longer than this will be truncated with a warning log
	maxURLLength = 2048
	// urlPreviewLength is the maximum length for URL preview in log messages
	urlPreviewLength = 100
)

const (
	// Default poll interval
	defaultPollIntervalSeconds = 30
)

// ElasticsearchClient defines the interface for ES operations
type ElasticsearchClient interface {
	// QueryRawContent queries for raw content with pending classification status
	QueryRawContent(ctx context.Context, status string, batchSize int) ([]*domain.RawContent, error)

	// IndexClassifiedContent indexes classified content
	IndexClassifiedContent(ctx context.Context, content *domain.ClassifiedContent) error

	// UpdateRawContentStatus updates the classification status of raw content
	UpdateRawContentStatus(ctx context.Context, contentID string, status string, classifiedAt time.Time) error

	// BulkIndexClassifiedContent indexes multiple classified content items
	BulkIndexClassifiedContent(ctx context.Context, contents []*domain.ClassifiedContent) error
}

// DatabaseClient defines the interface for database operations
type DatabaseClient interface {
	// SaveClassificationHistory saves classification result to history
	SaveClassificationHistory(ctx context.Context, history *domain.ClassificationHistory) error

	// SaveClassificationHistoryBatch saves multiple classification results
	SaveClassificationHistoryBatch(ctx context.Context, histories []*domain.ClassificationHistory) error
}

// Poller polls Elasticsearch for pending content and processes it
type Poller struct {
	esClient       ElasticsearchClient
	dbClient       DatabaseClient
	batchProcessor *BatchProcessor
	logger         Logger

	batchSize    int
	pollInterval time.Duration
	running      bool
	stopChan     chan struct{}
}

// PollerConfig holds poller configuration
type PollerConfig struct {
	BatchSize    int
	PollInterval time.Duration
}

// NewPoller creates a new poller
func NewPoller(
	esClient ElasticsearchClient,
	dbClient DatabaseClient,
	batchProcessor *BatchProcessor,
	logger Logger,
	config PollerConfig,
) *Poller {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.PollInterval <= 0 {
		config.PollInterval = defaultPollIntervalSeconds * time.Second
	}

	return &Poller{
		esClient:       esClient,
		dbClient:       dbClient,
		batchProcessor: batchProcessor,
		logger:         logger,
		batchSize:      config.BatchSize,
		pollInterval:   config.PollInterval,
		stopChan:       make(chan struct{}),
	}
}

// Start starts the poller
func (p *Poller) Start(ctx context.Context) error {
	if p.running {
		return errors.New("poller is already running")
	}

	p.running = true
	p.logger.Info("Poller starting",
		"batch_size", p.batchSize,
		"poll_interval", p.pollInterval,
	)

	go p.run(ctx)

	return nil
}

// Stop stops the poller
func (p *Poller) Stop() {
	if !p.running {
		return
	}

	p.logger.Info("Poller stopping")
	close(p.stopChan)
	p.running = false
}

// run is the main polling loop
func (p *Poller) run(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Process immediately on start
	if err := p.processPending(ctx); err != nil {
		p.logger.Error("Failed to process pending content on startup", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Poller stopped due to context cancellation")
			return
		case <-p.stopChan:
			p.logger.Info("Poller stopped")
			return
		case <-ticker.C:
			if err := p.processPending(ctx); err != nil {
				p.logger.Error("Failed to process pending content", "error", err)
			}
		}
	}
}

// processPending processes all pending content
func (p *Poller) processPending(ctx context.Context) error {
	p.logger.Debug("Polling for pending content", "batch_size", p.batchSize)

	// Query for pending raw content
	pendingItems, err := p.esClient.QueryRawContent(ctx, domain.StatusPending, p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to query pending content: %w", err)
	}

	if len(pendingItems) == 0 {
		p.logger.Debug("No pending content found")
		return nil
	}

	p.logger.Info("Found pending content", "count", len(pendingItems))

	// Process batch
	results, err := p.batchProcessor.Process(ctx, pendingItems)
	if err != nil {
		return fmt.Errorf("batch processing failed: %w", err)
	}

	// Index results
	if err = p.indexResults(ctx, results); err != nil {
		return fmt.Errorf("failed to index results: %w", err)
	}

	// Save to classification history
	if err = p.saveHistory(ctx, results); err != nil {
		p.logger.Warn("Failed to save classification history", "error", err)
		// Don't fail the whole operation if history save fails
	}

	return nil
}

// indexResults indexes classification results to Elasticsearch
func (p *Poller) indexResults(ctx context.Context, results []*ProcessResult) error {
	// Separate successful and failed results
	classifiedContents := make([]*domain.ClassifiedContent, 0, len(results))
	var failedContentIDs []string

	for _, result := range results {
		if result.Error != nil {
			failedContentIDs = append(failedContentIDs, result.Raw.ID)
			// Update status to failed
			if err := p.esClient.UpdateRawContentStatus(ctx, result.Raw.ID, domain.StatusFailed, time.Now()); err != nil {
				p.logger.Error("Failed to update status to failed",
					"content_id", result.Raw.ID,
					"error", err,
				)
			}
			continue
		}

		classifiedContents = append(classifiedContents, result.ClassifiedContent)
	}

	if len(failedContentIDs) > 0 {
		p.logger.Warn("Some items failed classification",
			"failed_count", len(failedContentIDs),
			"failed_ids", failedContentIDs,
		)
	}

	if len(classifiedContents) == 0 {
		return nil
	}

	// Bulk index classified content
	p.logger.Info("Indexing classified content", "count", len(classifiedContents))

	if err := p.esClient.BulkIndexClassifiedContent(ctx, classifiedContents); err != nil {
		return fmt.Errorf("bulk indexing failed: %w", err)
	}

	// Update raw content status to classified
	for _, content := range classifiedContents {
		if err := p.esClient.UpdateRawContentStatus(ctx, content.ID, domain.StatusClassified, time.Now()); err != nil {
			p.logger.Error("Failed to update raw content status",
				"content_id", content.ID,
				"error", err,
			)
			// Continue with next item
		}
	}

	p.logger.Info("Successfully indexed classified content", "count", len(classifiedContents))

	return nil
}

// validateURL validates and optionally truncates URLs to a reasonable length
// This is defensive programming - the database column is now TEXT, but we want
// to log warnings for extremely long URLs and prevent potential issues
func (p *Poller) validateURL(url string) string {
	if len(url) <= maxURLLength {
		return url
	}

	// Truncate URL and log warning
	truncated := url[:maxURLLength]

	// Determine preview length (use shorter of URL length or preview limit)
	previewLen := len(url)
	if previewLen > urlPreviewLength {
		previewLen = urlPreviewLength
	}

	p.logger.Warn("URL truncated for classification history",
		"original_length", len(url),
		"truncated_length", maxURLLength,
		"url_preview", url[:previewLen],
	)
	return truncated
}

// saveHistory saves classification results to database for ML training
func (p *Poller) saveHistory(ctx context.Context, results []*ProcessResult) error {
	histories := make([]*domain.ClassificationHistory, 0, len(results))

	for _, result := range results {
		if result.Error != nil || result.ClassificationResult == nil {
			continue
		}

		history := &domain.ClassificationHistory{
			ContentID:             result.Raw.ID,
			ContentURL:            p.validateURL(result.Raw.URL),
			SourceName:            result.Raw.SourceName,
			ContentType:           result.ClassificationResult.ContentType,
			ContentSubtype:        result.ClassificationResult.ContentSubtype,
			QualityScore:          result.ClassificationResult.QualityScore,
			Topics:                result.ClassificationResult.Topics,
			SourceReputationScore: result.ClassificationResult.SourceReputation,
			ClassifierVersion:     result.ClassificationResult.ClassifierVersion,
			ClassificationMethod:  result.ClassificationResult.ClassificationMethod,
			ModelVersion:          result.ClassificationResult.ModelVersion,
			Confidence:            result.ClassificationResult.Confidence,
			ProcessingTimeMs:      int(result.ClassificationResult.ProcessingTimeMs),
			ClassifiedAt:          result.ClassificationResult.ClassifiedAt,
		}

		histories = append(histories, history)
	}

	if len(histories) == 0 {
		return nil
	}

	p.logger.Debug("Saving classification history", "count", len(histories))

	if err := p.dbClient.SaveClassificationHistoryBatch(ctx, histories); err != nil {
		return fmt.Errorf("failed to save history batch: %w", err)
	}

	return nil
}

// IsRunning returns whether the poller is currently running
func (p *Poller) IsRunning() bool {
	return p.running
}

// GetStats returns poller statistics
func (p *Poller) GetStats() map[string]any {
	return map[string]any{
		"running":       p.running,
		"batch_size":    p.batchSize,
		"poll_interval": p.pollInterval.String(),
	}
}

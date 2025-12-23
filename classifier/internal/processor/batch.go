package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// BatchProcessor processes multiple content items in parallel using a worker pool
type BatchProcessor struct {
	classifier  *classifier.Classifier
	concurrency int
	logger      Logger
}

// Logger defines the logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// ProcessResult holds the result of processing a single item
type ProcessResult struct {
	Raw                *domain.RawContent
	ClassificationResult *domain.ClassificationResult
	ClassifiedContent  *domain.ClassifiedContent
	Error              error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(classifier *classifier.Classifier, concurrency int, logger Logger) *BatchProcessor {
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}

	return &BatchProcessor{
		classifier:  classifier,
		concurrency: concurrency,
		logger:      logger,
	}
}

// Process processes a batch of raw content items using worker pool
func (b *BatchProcessor) Process(ctx context.Context, rawItems []*domain.RawContent) ([]*ProcessResult, error) {
	if len(rawItems) == 0 {
		return []*ProcessResult{}, nil
	}

	b.logger.Info("Starting batch processing",
		"batch_size", len(rawItems),
		"concurrency", b.concurrency,
	)

	startTime := time.Now()

	// Create channels
	jobs := make(chan *domain.RawContent, len(rawItems))
	results := make(chan *ProcessResult, len(rawItems))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < b.concurrency; i++ {
		wg.Add(1)
		go b.worker(ctx, i, jobs, results, &wg)
	}

	// Send jobs to workers
	for _, raw := range rawItems {
		jobs <- raw
	}
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()
	close(results)

	// Collect results
	processResults := make([]*ProcessResult, 0, len(rawItems))
	for result := range results {
		processResults = append(processResults, result)
	}

	duration := time.Since(startTime)
	successCount := 0
	errorCount := 0

	for _, result := range processResults {
		if result.Error == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	b.logger.Info("Batch processing complete",
		"total", len(rawItems),
		"success", successCount,
		"errors", errorCount,
		"duration_ms", duration.Milliseconds(),
		"items_per_second", float64(len(rawItems))/duration.Seconds(),
	)

	return processResults, nil
}

// worker processes items from the jobs channel
func (b *BatchProcessor) worker(
	ctx context.Context,
	id int,
	jobs <-chan *domain.RawContent,
	results chan<- *ProcessResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	b.logger.Debug("Worker started", "worker_id", id)

	for raw := range jobs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			b.logger.Warn("Worker stopping due to context cancellation", "worker_id", id)
			return
		default:
		}

		result := b.processItem(ctx, raw)
		results <- result
	}

	b.logger.Debug("Worker finished", "worker_id", id)
}

// processItem processes a single content item
func (b *BatchProcessor) processItem(ctx context.Context, raw *domain.RawContent) *ProcessResult {
	result := &ProcessResult{
		Raw: raw,
	}

	// Classify the content
	classificationResult, err := b.classifier.Classify(ctx, raw)
	if err != nil {
		result.Error = fmt.Errorf("classification failed: %w", err)
		b.logger.Error("Failed to classify content",
			"content_id", raw.ID,
			"error", err,
		)
		return result
	}

	result.ClassificationResult = classificationResult

	// Build classified content for indexing
	classifiedContent := b.classifier.BuildClassifiedContent(raw, classificationResult)
	result.ClassifiedContent = classifiedContent

	b.logger.Debug("Item processed successfully",
		"content_id", raw.ID,
		"content_type", classificationResult.ContentType,
		"quality_score", classificationResult.QualityScore,
	)

	return result
}

// GetStats returns statistics about the batch processor
func (b *BatchProcessor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"concurrency": b.concurrency,
	}
}

// SetConcurrency updates the worker pool concurrency
func (b *BatchProcessor) SetConcurrency(concurrency int) {
	if concurrency > 0 {
		b.concurrency = concurrency
		b.logger.Info("Concurrency updated", "new_concurrency", concurrency)
	}
}

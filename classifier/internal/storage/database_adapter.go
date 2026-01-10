package storage

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// urlLogTruncateLength is the maximum length for URLs in log messages
	urlLogTruncateLength = 100
)

// HistoryRepository defines the interface for classification history operations
// This allows for easier testing with mocks
type HistoryRepository interface {
	Create(ctx context.Context, history *domain.ClassificationHistory) error
}

// DatabaseAdapter adapts the ClassificationHistoryRepository to the DatabaseClient interface
type DatabaseAdapter struct {
	historyRepo HistoryRepository
	logger      infralogger.Logger
}

// NewDatabaseAdapter creates a new database adapter
func NewDatabaseAdapter(historyRepo *database.ClassificationHistoryRepository) *DatabaseAdapter {
	return &DatabaseAdapter{
		historyRepo: historyRepo,
		logger:      nil, // Logger is optional for backward compatibility
	}
}

// NewDatabaseAdapterWithLogger creates a new database adapter with a logger
func NewDatabaseAdapterWithLogger(historyRepo *database.ClassificationHistoryRepository, logger infralogger.Logger) *DatabaseAdapter {
	return &DatabaseAdapter{
		historyRepo: historyRepo,
		logger:      logger,
	}
}

// NewDatabaseAdapterWithRepository creates a new database adapter with a custom repository (for testing)
func NewDatabaseAdapterWithRepository(historyRepo HistoryRepository, logger infralogger.Logger) *DatabaseAdapter {
	return &DatabaseAdapter{
		historyRepo: historyRepo,
		logger:      logger,
	}
}

// SaveClassificationHistory saves a single classification result to history
func (d *DatabaseAdapter) SaveClassificationHistory(ctx context.Context, history *domain.ClassificationHistory) error {
	return d.historyRepo.Create(ctx, history)
}

// SaveClassificationHistoryBatch saves multiple classification results
func (d *DatabaseAdapter) SaveClassificationHistoryBatch(ctx context.Context, histories []*domain.ClassificationHistory) error {
	if len(histories) == 0 {
		return nil
	}

	var failedCount int
	var firstError error
	var failedContentIDs []string

	// The repository doesn't have batch insert, so we'll insert one by one
	// In a real implementation, you might want to add a batch insert method
	for _, history := range histories {
		if err := d.historyRepo.Create(ctx, history); err != nil {
			failedCount++
			if firstError == nil {
				firstError = err
			}
			failedContentIDs = append(failedContentIDs, history.ContentID)

			// Log each individual error if logger is available
			if d.logger != nil {
				d.logger.Error("Failed to save classification history record",
					infralogger.String("content_id", history.ContentID),
					infralogger.String("content_url", truncateString(history.ContentURL, urlLogTruncateLength)),
					infralogger.Error(err),
				)
			}
		}
	}

	// If all records failed, return the first error
	if failedCount == len(histories) {
		if d.logger != nil {
			d.logger.Error("All classification history records failed to save",
				infralogger.Int("total_count", len(histories)),
				infralogger.Int("failed_count", failedCount),
				infralogger.Error(firstError),
			)
		}
		return fmt.Errorf("all %d classification history records failed: %w", failedCount, firstError)
	}

	// If some records failed, log a warning but don't fail the entire operation
	if failedCount > 0 {
		if d.logger != nil {
			d.logger.Warn("Some classification history records failed to save",
				infralogger.Int("total_count", len(histories)),
				infralogger.Int("success_count", len(histories)-failedCount),
				infralogger.Int("failed_count", failedCount),
				infralogger.Any("failed_content_ids", failedContentIDs),
				infralogger.Error(firstError),
			)
		}
		// Return nil to allow processing to continue, but log the partial failure
		// This maintains backward compatibility while providing visibility
	}

	return nil
}

// truncateString truncates a string to a maximum length for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

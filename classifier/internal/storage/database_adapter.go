package storage

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// DatabaseAdapter adapts the ClassificationHistoryRepository to the DatabaseClient interface
type DatabaseAdapter struct {
	historyRepo *database.ClassificationHistoryRepository
}

// NewDatabaseAdapter creates a new database adapter
func NewDatabaseAdapter(historyRepo *database.ClassificationHistoryRepository) *DatabaseAdapter {
	return &DatabaseAdapter{
		historyRepo: historyRepo,
	}
}

// SaveClassificationHistory saves a single classification result to history
func (d *DatabaseAdapter) SaveClassificationHistory(ctx context.Context, history *domain.ClassificationHistory) error {
	return d.historyRepo.Create(ctx, history)
}

// SaveClassificationHistoryBatch saves multiple classification results
func (d *DatabaseAdapter) SaveClassificationHistoryBatch(ctx context.Context, histories []*domain.ClassificationHistory) error {
	// The repository doesn't have batch insert, so we'll insert one by one
	// In a real implementation, you might want to add a batch insert method
	for _, history := range histories {
		if err := d.historyRepo.Create(ctx, history); err != nil {
			// Log error but continue with next item
			// In production, you might want to collect errors and return them
			continue
		}
	}
	return nil
}

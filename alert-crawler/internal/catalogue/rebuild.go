package catalogue

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// ESActiveAlertQuerier is the narrow interface that RebuildFromES accepts.
// It is satisfied by the elasticsearch package's Client without importing it directly,
// keeping catalogue at L1 and free of ES-package coupling.
type ESActiveAlertQuerier interface {
	QueryActiveAlertIDs(ctx context.Context) ([]domain.Alert, error)
}

// RebuildFromES populates the alert_catalogue from Elasticsearch active alerts.
// It is idempotent: rows are inserted via INSERT OR REPLACE so re-running is safe.
// It does NOT emit lifecycle events; callers must handle downstream side effects.
func (s *Store) RebuildFromES(ctx context.Context, esClient ESActiveAlertQuerier) error {
	alerts, err := esClient.QueryActiveAlertIDs(ctx)
	if err != nil {
		return fmt.Errorf("catalogue: rebuild from ES query: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for i := range alerts {
		a := &alerts[i]

		sourceID := sourceIDFromAlert(a)

		const q = `
			INSERT OR REPLACE INTO alert_catalogue
			    (source_id, alert_id, last_seen_at, is_active, content_hash)
			VALUES (?, ?, ?, 1, '')`

		if _, execErr := s.db.ExecContext(ctx, q, sourceID, a.ID, now); execErr != nil {
			return fmt.Errorf("catalogue: rebuild insert alert %s: %w", a.ID, execErr)
		}
	}

	return nil
}

// sourceIDFromAlert extracts the source_id from an alert's first attribution.
// Falls back to an empty string when no attribution is present.
func sourceIDFromAlert(a *domain.Alert) string {
	if len(a.Sources) > 0 {
		return a.Sources[0].SourceID
	}

	return ""
}

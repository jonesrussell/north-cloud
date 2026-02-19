package bootstrap

import (
	"context"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// loggingFrontierRepo wraps FrontierRepository and logs every Submit for Grafana/Loki.
// Used for API handler, feed poller, and link handler; the worker pool uses the raw repo.
type loggingFrontierRepo struct {
	repo *database.FrontierRepository
	log  infralogger.Logger
}

// NewLoggingFrontierRepo returns a wrapper that implements the handler/submitter surface
// and logs "URL submitted to frontier" with url, source_id, origin for each Submit.
func NewLoggingFrontierRepo(repo *database.FrontierRepository, log infralogger.Logger) *loggingFrontierRepo {
	return &loggingFrontierRepo{repo: repo, log: log}
}

// Submit delegates to the inner repo and logs one line for dashboard LogQL.
func (w *loggingFrontierRepo) Submit(ctx context.Context, params database.SubmitParams) error {
	if err := w.repo.Submit(ctx, params); err != nil {
		return err
	}
	w.log.Info("URL submitted to frontier",
		infralogger.String("url", params.URL),
		infralogger.String("source_id", params.SourceID),
		infralogger.String("origin", params.Origin),
	)
	return nil
}

// List delegates to the inner repository.
func (w *loggingFrontierRepo) List(ctx context.Context, filters database.FrontierFilters) ([]*domain.FrontierURL, int, error) {
	return w.repo.List(ctx, filters)
}

// Stats delegates to the inner repository.
func (w *loggingFrontierRepo) Stats(ctx context.Context) (*database.FrontierStats, error) {
	return w.repo.Stats(ctx)
}

// ResetForRetry delegates to the inner repository.
func (w *loggingFrontierRepo) ResetForRetry(ctx context.Context, id string) error {
	return w.repo.ResetForRetry(ctx, id)
}

// Delete delegates to the inner repository.
func (w *loggingFrontierRepo) Delete(ctx context.Context, id string) error {
	return w.repo.Delete(ctx, id)
}

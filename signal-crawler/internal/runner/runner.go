package runner

import (
	"context"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
)

// Dedup is the interface for the deduplication store.
type Dedup interface {
	Seen(ctx context.Context, source, externalID string) (bool, error)
	Mark(ctx context.Context, source, externalID string) error
}

// Ingest is the interface for the NorthOps ingest client.
type Ingest interface {
	Post(ctx context.Context, sig adapter.Signal) error
}

// Stats holds per-source run statistics.
type Stats struct {
	Source   string
	Scanned  int
	Ingested int
	Skipped  int
	Errors   int
}

// Runner orchestrates the scan → dedup → ingest pipeline.
type Runner struct {
	sources []adapter.Source
	dedup   Dedup
	ingest  Ingest
	dryRun  bool
	log     infralogger.Logger
}

// New creates a Runner with the given sources, dedup store, ingest client, dry-run flag, and logger.
func New(sources []adapter.Source, dedup Dedup, ingest Ingest, dryRun bool, log infralogger.Logger) *Runner {
	return &Runner{
		sources: sources,
		dedup:   dedup,
		ingest:  ingest,
		dryRun:  dryRun,
		log:     log,
	}
}

// Run executes the scan → dedup → ingest pipeline for each source and returns per-source stats.
func (r *Runner) Run(ctx context.Context) []Stats {
	results := make([]Stats, 0, len(r.sources))

	for _, src := range r.sources {
		s := Stats{Source: src.Name()}

		signals, err := src.Scan(ctx)
		if err != nil {
			r.log.Warn("scan failed", infralogger.String("source", src.Name()), infralogger.Error(err))
			s.Errors++
			results = append(results, s)
			continue
		}

		s.Scanned = len(signals)

		for _, sig := range signals {
			seen, err := r.dedup.Seen(ctx, src.Name(), sig.ExternalID)
			if err != nil {
				r.log.Warn("dedup check failed", infralogger.String("source", src.Name()), infralogger.Error(err))
				s.Errors++
				continue
			}

			if seen {
				s.Skipped++
				continue
			}

			// Not seen: ingest unless dry-run.
			if !r.dryRun {
				if err := r.ingest.Post(ctx, sig); err != nil {
					r.log.Warn("ingest failed", infralogger.String("source", src.Name()), infralogger.Error(err))
					s.Errors++
					continue
				}
				if err := r.dedup.Mark(ctx, src.Name(), sig.ExternalID); err != nil {
					r.log.Warn("dedup mark failed", infralogger.String("source", src.Name()), infralogger.Error(err))
					s.Errors++
					continue
				}
			}

			s.Ingested++
		}

		results = append(results, s)
	}

	return results
}

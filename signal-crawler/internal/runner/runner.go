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

// processSignal handles dedup check and ingest for a single signal, updating s in place.
func (r *Runner) processSignal(ctx context.Context, src adapter.Source, sig adapter.Signal, s *Stats) {
	seen, err := r.dedup.Seen(ctx, src.Name(), sig.ExternalID)
	if err != nil {
		r.log.Warn("dedup check failed", infralogger.String("source", src.Name()), infralogger.Error(err))
		s.Errors++
		return
	}

	if seen {
		s.Skipped++
		return
	}

	if !r.dryRun {
		err = r.ingest.Post(ctx, sig)
		if err != nil {
			r.log.Warn("ingest failed", infralogger.String("source", src.Name()), infralogger.Error(err))
			s.Errors++
			return
		}
		err = r.dedup.Mark(ctx, src.Name(), sig.ExternalID)
		if err != nil {
			r.log.Warn("dedup mark failed", infralogger.String("source", src.Name()), infralogger.Error(err))
			s.Errors++
			return
		}
	}

	s.Ingested++
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
			r.processSignal(ctx, src, sig, &s)
		}

		results = append(results, s)
	}

	return results
}

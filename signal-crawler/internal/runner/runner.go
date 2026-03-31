package runner

import (
	"context"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
)

// Dedup is the interface for the deduplication store.
type Dedup interface {
	Seen(source, externalID string) (bool, error)
	Mark(source, externalID string) error
}

// Ingest is the interface for the NorthOps ingest client.
type Ingest interface {
	Post(sig adapter.Signal) error
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
}

// New creates a Runner with the given sources, dedup store, ingest client, and dry-run flag.
func New(sources []adapter.Source, dedup Dedup, ingest Ingest, dryRun bool) *Runner {
	return &Runner{
		sources: sources,
		dedup:   dedup,
		ingest:  ingest,
		dryRun:  dryRun,
	}
}

// Run executes the scan → dedup → ingest pipeline for each source and returns per-source stats.
func (r *Runner) Run(ctx context.Context) []Stats {
	results := make([]Stats, 0, len(r.sources))

	for _, src := range r.sources {
		s := Stats{Source: src.Name()}

		signals, err := src.Scan(ctx)
		if err != nil {
			s.Errors++
			results = append(results, s)
			continue
		}

		s.Scanned = len(signals)

		for _, sig := range signals {
			seen, err := r.dedup.Seen(src.Name(), sig.ExternalID)
			if err != nil {
				s.Errors++
				continue
			}

			if seen {
				s.Skipped++
				continue
			}

			// Not seen: ingest unless dry-run.
			if !r.dryRun {
				if err := r.ingest.Post(sig); err != nil {
					s.Errors++
					continue
				}
				if err := r.dedup.Mark(src.Name(), sig.ExternalID); err != nil {
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

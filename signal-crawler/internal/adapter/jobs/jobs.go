package jobs

import (
	"context"
	"errors"
	"fmt"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/scoring"
)

// Posting represents a single job listing extracted by a board parser.
type Posting struct {
	Title   string
	Company string
	URL     string
	ID      string
	Body    string
	Sector  string
}

// Board fetches job postings from a single source.
type Board interface {
	Name() string
	Fetch(ctx context.Context) ([]Posting, error)
}

// Adapter scans multiple job boards for infrastructure-intent signals.
type Adapter struct {
	boards []Board
	log    infralogger.Logger
}

// New creates a new jobs Adapter that aggregates signals from multiple boards.
func New(boards []Board, log infralogger.Logger) *Adapter {
	return &Adapter{boards: boards, log: log}
}

// Name returns the short identifier for this adapter.
func (a *Adapter) Name() string { return "jobs" }

// Scan fetches postings from all boards, scores them, and returns matching signals.
// Continues on per-board errors, returning partial results with a combined error.
func (a *Adapter) Scan(ctx context.Context) ([]adapter.Signal, error) {
	var allSignals []adapter.Signal
	var errs []error

	for _, board := range a.boards {
		postings, err := board.Fetch(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("jobs: board %s: %w", board.Name(), err))
			if a.log != nil {
				a.log.Warn("board fetch failed",
					infralogger.String("board", board.Name()),
					infralogger.Error(err),
				)
			}
			continue
		}

		matched := 0
		for _, p := range postings {
			combined := p.Title + " " + p.Body
			score, phrase := scoring.Score(combined)
			if score == 0 {
				continue
			}

			label := p.Title
			if p.Company != "" {
				label = p.Company + " — " + p.Title
			}

			sector := p.Sector
			if sector == "" {
				sector = "tech"
			}

			allSignals = append(allSignals, adapter.Signal{
				SignalType:     "job_posting",
				SourceName:     board.Name(),
				Label:          label,
				SourceURL:      p.URL,
				ExternalID:     board.Name() + "|" + p.ID,
				SignalStrength: score,
				Sector:         sector,
				Notes:          fmt.Sprintf("Matched: %s (via %s)", phrase, board.Name()),
			})
			matched++
		}

		if a.log != nil {
			a.log.Info("board scan complete",
				infralogger.String("board", board.Name()),
				infralogger.Int("total", len(postings)),
				infralogger.Int("matched", matched),
			)
		}
	}

	return allSignals, errors.Join(errs...)
}

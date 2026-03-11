package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// ScraperConfig holds the config needed to run leadership scrape jobs.
type ScraperConfig struct {
	SourceManagerURL string
	JWTToken         string
}

// RunLeadershipScrapeJob runs the leadership scraper for a scheduled job.
func RunLeadershipScrapeJob(ctx context.Context, cfg ScraperConfig, logger infralogger.Logger) error {
	if cfg.SourceManagerURL == "" {
		return errors.New("leadership scrape: source-manager URL not configured")
	}

	s := scraper.New(scraper.Config{
		SourceManagerURL: cfg.SourceManagerURL,
		JWTToken:         cfg.JWTToken,
	}, logger)

	results, err := s.Run(ctx)
	if err != nil {
		return fmt.Errorf("leadership scrape: %w", err)
	}

	var totalPeople, totalOffices, totalErrors int
	for _, r := range results {
		totalPeople += r.PeopleAdded
		if r.OfficeUpdated {
			totalOffices++
		}
		if r.Error != "" {
			totalErrors++
		}
	}

	logger.Info("leadership scrape completed",
		infralogger.Int("communities_processed", len(results)),
		infralogger.Int("people_added", totalPeople),
		infralogger.Int("offices_updated", totalOffices),
		infralogger.Int("errors", totalErrors),
	)

	return nil
}

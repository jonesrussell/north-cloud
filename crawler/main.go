package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// defaultWorkerCount is the default number of concurrent scraper workers.
const defaultWorkerCount = 4

func main() {
	if len(os.Args) > 1 && os.Args[1] == "scrape-leadership" {
		if err := runScrapeLeadership(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScrapeLeadership(args []string) error {
	fs := flag.NewFlagSet("scrape-leadership", flag.ExitOnError)
	communityID := fs.String("community-id", "", "Scrape a single community by ID")
	dryRun := fs.Bool("dry-run", false, "Print extracted data without saving")
	workers := fs.Int("workers", defaultWorkerCount, "Number of concurrent workers")
	sourceManagerURL := fs.String(
		"source-manager-url", "",
		"Source-manager base URL (overrides config)",
	)
	jwtToken := fs.String("jwt-token", "", "JWT token for authenticated API calls")

	if parseErr := fs.Parse(args); parseErr != nil {
		return fmt.Errorf("parse flags: %w", parseErr)
	}

	// Load config for defaults
	cfgPath := infraconfig.GetConfigPath("config.yml")

	cfg, cfgErr := crawlerconfig.Load(cfgPath)
	if cfgErr != nil {
		return fmt.Errorf("load config: %w", cfgErr)
	}

	smURL := cfg.GetSourceManagerConfig().URL
	if *sourceManagerURL != "" {
		smURL = *sourceManagerURL
	}

	log, logErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if logErr != nil {
		return fmt.Errorf("create logger: %w", logErr)
	}
	defer func() { _ = log.Sync() }()

	s := scraper.New(scraper.Config{
		SourceManagerURL: smURL,
		JWTToken:         *jwtToken,
		Workers:          *workers,
		DryRun:           *dryRun,
		CommunityID:      *communityID,
	}, log)

	results, err := s.Run(context.Background())
	if err != nil {
		return fmt.Errorf("run leadership scraper: %w", err)
	}

	if *dryRun {
		if printErr := scraper.PrintDryRunResults(results); printErr != nil {
			return fmt.Errorf("print dry-run results: %w", printErr)
		}
		return nil
	}

	printSummary(results, log)
	return nil
}

func printSummary(results []scraper.Result, log infralogger.Logger) {
	var totalPeopleAdded, totalErrors int
	var totalOfficesUpdated int

	for _, r := range results {
		totalPeopleAdded += r.PeopleAdded
		if r.OfficeUpdated {
			totalOfficesUpdated++
		}
		if r.Error != "" {
			totalErrors++
		}
	}

	log.Info("scrape complete",
		infralogger.Int("communities_processed", len(results)),
		infralogger.Int("people_added", totalPeopleAdded),
		infralogger.Int("offices_updated", totalOfficesUpdated),
		infralogger.Int("errors", totalErrors),
	)
}

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/seeder"
)

const (
	sourceCIRNAC   = "cirnac"
	sourceStatscan = "statscan"
)

func main() {
	source := flag.String("source", "", "Data source to refresh: cirnac or statscan (required)")
	filePath := flag.String("file", "", "Path to data file (required)")
	province := flag.String("province", "", "Province filter for statscan (e.g. ON)")
	dryRun := flag.Bool("dry-run", false, "Preview changes without saving")
	flag.Parse()

	if *source == "" || *filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: refresh-communities --source=cirnac|statscan --file=<path> [--province=ON] [--dry-run]")
		os.Exit(1)
	}

	if *source != sourceCIRNAC && *source != sourceStatscan {
		fmt.Fprintf(os.Stderr, "Unknown source %q: must be %q or %q\n", *source, sourceCIRNAC, sourceStatscan)
		os.Exit(1)
	}

	if err := run(*source, *filePath, *province, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(source, filePath, province string, dryRun bool) error {
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, logErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "console",
	})
	if logErr != nil {
		return fmt.Errorf("create logger: %w", logErr)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	db, dbErr := sql.Open("postgres", dsn)
	if dbErr != nil {
		return fmt.Errorf("connect to database: %w", dbErr)
	}
	defer db.Close()

	ctx := context.Background()
	if pingErr := db.PingContext(ctx); pingErr != nil {
		return fmt.Errorf("ping database: %w", pingErr)
	}

	repo := repository.NewCommunityRepository(db, log)

	switch source {
	case sourceCIRNAC:
		return refreshCIRNAC(ctx, repo, log, filePath, dryRun)
	case sourceStatscan:
		return refreshStatscan(ctx, repo, log, filePath, province, dryRun)
	default:
		return fmt.Errorf("unknown source: %s", source)
	}
}

func refreshCIRNAC(
	ctx context.Context,
	repo *repository.CommunityRepository,
	log infralogger.Logger,
	filePath string,
	dryRun bool,
) error {
	s := seeder.NewCIRNACSeeder(repo, log)
	result, err := s.SeedFromFile(ctx, filePath, dryRun)
	if err != nil {
		return fmt.Errorf("CIRNAC refresh: %w", err)
	}

	printCIRNACResult(result, dryRun)
	return nil
}

func refreshStatscan(
	ctx context.Context,
	repo *repository.CommunityRepository,
	log infralogger.Logger,
	filePath, province string,
	dryRun bool,
) error {
	s := seeder.NewGeoNamesSeeder(repo, log, province)
	result, err := s.SeedFromFile(ctx, filePath, dryRun)
	if err != nil {
		return fmt.Errorf("StatsCan refresh: %w", err)
	}

	printStatscanResult(result, province, dryRun)
	return nil
}

func printCIRNACResult(result *seeder.CIRNACResult, dryRun bool) {
	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("\n[%s] CIRNAC Refresh Complete\n", mode)
	fmt.Printf("  Total rows:  %d\n", result.Total)
	if dryRun {
		fmt.Printf("  Would create: %d\n", result.WouldCreate)
	} else {
		fmt.Printf("  Created:     %d\n", result.Created)
	}
	fmt.Printf("  Skipped:     %d\n", result.Skipped)
	fmt.Printf("  Errors:      %d\n", result.Errors)
}

func printStatscanResult(result *seeder.GeoNamesResult, province string, dryRun bool) {
	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("\n[%s] StatsCan Refresh Complete\n", mode)
	if province != "" {
		fmt.Printf("  Province:    %s\n", province)
	}
	fmt.Printf("  Processed:   %d\n", result.Total)
	if dryRun {
		fmt.Printf("  Would create: %d\n", result.WouldCreate)
	} else {
		fmt.Printf("  Created:     %d\n", result.Created)
	}
	fmt.Printf("  Skipped:     %d\n", result.Skipped)
	fmt.Printf("  Errors:      %d\n", result.Errors)
}

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

func main() {
	csvPath := flag.String("csv", "", "Path to CIRNAC CSV file (required)")
	dryRun := flag.Bool("dry-run", false, "Preview changes without saving")
	flag.Parse()

	if *csvPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: seed-communities --csv=<path> [--dry-run]")
		os.Exit(1)
	}

	if err := run(*csvPath, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(csvPath string, dryRun bool) error {
	// Load config
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create logger
	log, logErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "console",
	})
	if logErr != nil {
		return fmt.Errorf("create logger: %w", logErr)
	}

	// Connect to database
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

	// Run seeder
	repo := repository.NewCommunityRepository(db, log)
	s := seeder.NewCIRNACSeeder(repo, log)

	result, seedErr := s.SeedFromFile(ctx, csvPath, dryRun)
	if seedErr != nil {
		return fmt.Errorf("seed communities: %w", seedErr)
	}

	// Print summary
	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("\n[%s] CIRNAC Seed Complete\n", mode)
	fmt.Printf("  Total rows:  %d\n", result.Total)
	if dryRun {
		fmt.Printf("  Would create: %d\n", result.WouldCreate)
	} else {
		fmt.Printf("  Created:     %d\n", result.Created)
	}
	fmt.Printf("  Skipped:     %d\n", result.Skipped)
	fmt.Printf("  Errors:      %d\n", result.Errors)

	return nil
}

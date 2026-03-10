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
	filePath := flag.String("file", "", "Path to GeoNames CA.txt file (required)")
	province := flag.String("province", "", "Filter to a single province code (e.g. ON)")
	dryRun := flag.Bool("dry-run", false, "Preview changes without saving")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: seed-municipalities --file=CA.txt [--province=ON] [--dry-run]")
		os.Exit(1)
	}

	if err := run(*filePath, *province, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(filePath, province string, dryRun bool) error {
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
	s := seeder.NewGeoNamesSeeder(repo, log, province)

	result, seedErr := s.SeedFromFile(ctx, filePath, dryRun)
	if seedErr != nil {
		return fmt.Errorf("seed municipalities: %w", seedErr)
	}

	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}

	fmt.Printf("\n[%s] GeoNames Municipal Seed Complete\n", mode)
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

	return nil
}

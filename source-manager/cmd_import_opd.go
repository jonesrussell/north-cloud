package main

import (
	"context"
	"errors"
	"flag"
	"fmt"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	defaultBatchSize     = 500
	importCommandVersion = "dev"
)

func runImportOPD(args []string) error {
	fs := flag.NewFlagSet("import-opd", flag.ExitOnError)
	filePath := fs.String("file", "", "Path to OPD JSONL file (required)")
	batchSize := fs.Int("batch-size", defaultBatchSize, "Entries per DB batch")
	dryRun := fs.Bool("dry-run", false, "Validate without writing to DB")
	consentPublicDisplay := fs.Bool(
		"consent-public-display",
		false,
		"Set consent_public_display=true for imported entries",
	)

	if parseErr := fs.Parse(args); parseErr != nil {
		return fmt.Errorf("parse flags: %w", parseErr)
	}

	if *filePath == "" {
		fs.Usage()
		return errors.New("--file is required")
	}

	// Read and validate JSONL (no config needed for this step)
	entries, failures, readErr := importer.ReadOPDFile(*filePath)
	if readErr != nil {
		return fmt.Errorf("read file: %w", readErr)
	}

	if *dryRun {
		fmt.Printf("Dry run complete: %d valid, %d failed\n", len(entries), len(failures))
		return nil
	}

	if *consentPublicDisplay {
		for i := range entries {
			entries[i].ConsentPublicDisplay = true
		}
	}

	// Load config only when writing to DB
	cfg, cfgErr := config.Load(infraconfig.GetConfigPath("config.yml"))
	if cfgErr != nil {
		return fmt.Errorf("load config: %w", cfgErr)
	}
	if validationErr := cfg.Validate(); validationErr != nil {
		return fmt.Errorf("validate config: %w", validationErr)
	}

	log, logErr := bootstrap.CreateLogger(cfg, importCommandVersion)
	if logErr != nil {
		return fmt.Errorf("create logger: %w", logErr)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting OPD import",
		infralogger.String("file", *filePath),
		infralogger.Int("batch_size", *batchSize),
		infralogger.Bool("consent_public_display", *consentPublicDisplay),
		infralogger.Int("failures", len(failures)),
	)

	for i := range failures {
		log.Warn("Import failure",
			infralogger.Int("line", failures[i].Line),
			infralogger.String("reason", failures[i].Reason),
		)
	}

	return runImportOPDWrite(cfg, log, entries, failures, *batchSize)
}

func runImportOPDWrite(
	cfg *config.Config,
	log infralogger.Logger,
	entries []models.DictionaryEntry,
	failures []importer.ImportFailure,
	batchSize int,
) error {
	db, dbErr := bootstrap.SetupDatabase(cfg, log)
	if dbErr != nil {
		return fmt.Errorf("connect to database: %w", dbErr)
	}
	defer func() { _ = db.Close() }()

	dictRepo := repository.NewDictionaryRepository(db.DB(), log)
	ctx := context.Background()

	totalInserted, totalUpdated := 0, 0
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		inserted, updated := processBatch(ctx, dictRepo, log, entries[i:end], i)
		totalInserted += inserted
		totalUpdated += updated
	}

	fmt.Printf("Import complete: %d inserted, %d updated, %d failed\n",
		totalInserted, totalUpdated, len(failures))

	return nil
}

func processBatch(
	ctx context.Context,
	dictRepo *repository.DictionaryRepository,
	log infralogger.Logger,
	batch []models.DictionaryEntry,
	batchStart int,
) (inserted, updated int) {
	ins, upd, upsertErr := dictRepo.BulkUpsertEntries(ctx, batch)
	if upsertErr != nil {
		log.Warn("Batch failed, retrying",
			infralogger.Int("batch_start", batchStart),
			infralogger.Error(upsertErr),
		)
		ins, upd, upsertErr = dictRepo.BulkUpsertEntries(ctx, batch)
		if upsertErr != nil {
			log.Error("Batch failed after retry, skipping",
				infralogger.Int("batch_start", batchStart),
				infralogger.Error(upsertErr),
			)
			return 0, 0
		}
	}

	log.Info("Batch complete",
		infralogger.Int("batch_start", batchStart),
		infralogger.Int("inserted", ins),
		infralogger.Int("updated", upd),
	)
	return ins, upd
}

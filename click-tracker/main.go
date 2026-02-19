package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"

	_ "github.com/lib/pq"
)

// Database connection timeout.
const dbPingTimeout = 5 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	// Initialize logger
	log, err := createLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	// Connect to database
	db, err := connectDatabase(cfg, log)
	if err != nil {
		log.Error("Failed to connect to database", logger.Error(err))
		return 1
	}
	defer func() { _ = db.Close() }()

	// Run server (placeholder - wired in Task 8)
	return runServer(cfg, log, db)
}

// loadConfig loads and validates configuration.
func loadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if validationErr := cfg.Validate(); validationErr != nil {
		return nil, fmt.Errorf("validate config: %w", validationErr)
	}
	return cfg, nil
}

// createLogger creates a logger instance from configuration.
func createLogger(cfg *config.Config) (logger.Logger, error) {
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return log.With(logger.String("service", "click-tracker")), nil
}

// connectDatabase opens and verifies a database connection.
func connectDatabase(cfg *config.Config, log logger.Logger) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbPingTimeout)
	defer cancel()

	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", pingErr)
	}

	log.Info("Database connected",
		logger.String("host", cfg.Database.Host),
		logger.Int("port", cfg.Database.Port),
		logger.String("database", cfg.Database.Database),
	)

	return db, nil
}

// runServer is a placeholder that will be wired in Task 8.
func runServer(cfg *config.Config, log logger.Logger, _ *sql.DB) int {
	log.Info("Click-tracker service ready",
		logger.String("name", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Int("port", cfg.Service.Port),
		logger.Bool("debug", cfg.Service.Debug),
	)

	return 0
}

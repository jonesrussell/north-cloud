package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Exit codes for the migrate command.
const (
	exitSuccess = 0
	exitFailure = 1
)

// migrationsPath is the relative path to the migrations directory.
const migrationsPath = "file://migrations"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: migrate <up|down>")
		return exitFailure
	}

	direction := os.Args[1]
	if direction != "up" && direction != "down" {
		fmt.Fprintf(os.Stderr, "Invalid direction: %q (must be \"up\" or \"down\")\n", direction)
		return exitFailure
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return exitFailure
	}

	dsn := buildMigrateURL(cfg)

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create migrate instance: %v\n", err)
		return exitFailure
	}
	defer func() { _, _ = m.Close() }()

	if err := runMigration(m, direction); err != nil {
		fmt.Fprintf(os.Stderr, "Migration %s failed: %v\n", direction, err)
		return exitFailure
	}

	fmt.Printf("Migration %s completed successfully\n", direction)
	return exitSuccess
}

// loadConfig loads the application configuration.
func loadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}

// buildMigrateURL constructs a PostgreSQL URL from database config.
func buildMigrateURL(cfg *config.Config) string {
	db := &cfg.Database
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		db.User, db.Password, db.Host, db.Port, db.Database, db.SSLMode,
	)
}

// runMigration executes the migration in the specified direction.
func runMigration(m *migrate.Migrate, direction string) error {
	var err error

	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("No migrations to apply")
		return nil
	}

	return err
}

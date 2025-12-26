package database

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" //nolint:blankimports // File source driver
	"github.com/jonesrussell/auth/internal/config"
	"github.com/jonesrussell/auth/internal/logger"
)

// RunMigrations runs all pending migrations
func RunMigrations(cfg *config.Config, log logger.Logger) error {
	db, err := sql.Open("postgres", buildDSN(cfg))
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}

	// Use absolute path for migrations directory
	// In Docker, migrations are in /app/migrations
	// Locally, they're in ./migrations relative to the binary
	migrationsPath := "migrations"
	if absPath, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = absPath
	}
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("No pending migrations",
				logger.String("migrations_path", migrationsPath),
			)
			return nil
		}
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Info("Migrations applied successfully",
		logger.String("migrations_path", migrationsPath),
	)

	return nil
}

// MigrateDown rolls back N migrations (default: 1)
func MigrateDown(cfg *config.Config, steps int, log logger.Logger) error {
	db, err := sql.Open("postgres", buildDSN(cfg))
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}

	// Use absolute path for migrations directory
	// In Docker, migrations are in /app/migrations
	// Locally, they're in ./migrations relative to the binary
	migrationsPath := "migrations"
	if absPath, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = absPath
	}
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if steps <= 0 {
		steps = 1
	}

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("No migrations to rollback",
				logger.String("migrations_path", migrationsPath),
			)
			return nil
		}
		return fmt.Errorf("rollback migrations: %w", err)
	}

	log.Info("Migrations rolled back successfully",
		logger.String("migrations_path", migrationsPath),
		logger.Int("steps", steps),
	)

	return nil
}

// GetMigrationVersion returns the current migration version
func GetMigrationVersion(cfg *config.Config, log logger.Logger) (uint, bool, error) {
	db, err := sql.Open("postgres", buildDSN(cfg))
	if err != nil {
		return 0, false, fmt.Errorf("open database connection: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("create postgres driver: %w", err)
	}

	// Use absolute path for migrations directory
	// In Docker, migrations are in /app/migrations
	// Locally, they're in ./migrations relative to the binary
	migrationsPath := "migrations"
	if absPath, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = absPath
	}
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return 0, false, fmt.Errorf("create migrate instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("get migration version: %w", err)
	}

	return version, dirty, nil
}

// ForceMigrationVersion forces the migration version (for fixing dirty state)
func ForceMigrationVersion(cfg *config.Config, version int, log logger.Logger) error {
	db, err := sql.Open("postgres", buildDSN(cfg))
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}

	// Use absolute path for migrations directory
	// In Docker, migrations are in /app/migrations
	// Locally, they're in ./migrations relative to the binary
	migrationsPath := "migrations"
	if absPath, err := filepath.Abs(migrationsPath); err == nil {
		migrationsPath = absPath
	}
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force migration version: %w", err)
	}

	log.Info("Migration version forced",
		logger.String("migrations_path", migrationsPath),
		logger.Int("version", version),
	)

	return nil
}

func buildDSN(cfg *config.Config) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)
}


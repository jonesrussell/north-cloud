package testhelpers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jonesrussell/gosources/internal/logger"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
)

// TestDatabase provides helper functions for test database setup
// For integration tests, you can use testcontainers or a local PostgreSQL instance
// See internal/repository/source_test.go for an example of using a local test database

// RunMigrations executes SQL migration files on a database connection
func RunMigrations(ctx context.Context, db *sql.DB, log logger.Logger) error {
	// Get the path to migrations directory
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	migrationFile := filepath.Join(migrationsPath, "001_create_sources_table.sql")
	sqlBytes, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	if _, execErr := db.ExecContext(ctx, string(sqlBytes)); execErr != nil {
		return fmt.Errorf("execute migration: %w", execErr)
	}

	if log != nil {
		log.Info("Migrations applied successfully",
			logger.String("migration_file", migrationFile),
		)
	}

	return nil
}

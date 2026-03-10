package testhelpers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
)

// RunMigrations executes all SQL migration files on a database connection.
func RunMigrations(ctx context.Context, db *sql.DB, log infralogger.Logger) error {
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	files, globErr := filepath.Glob(filepath.Join(migrationsPath, "*.up.sql"))
	if globErr != nil {
		return fmt.Errorf("glob migrations: %w", globErr)
	}

	sort.Strings(files)

	for _, f := range files {
		sqlBytes, readErr := os.ReadFile(f)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", filepath.Base(f), readErr)
		}

		if _, execErr := db.ExecContext(ctx, string(sqlBytes)); execErr != nil {
			return fmt.Errorf("execute migration %s: %w", filepath.Base(f), execErr)
		}
	}

	if log != nil {
		log.Info("Migrations applied successfully",
			infralogger.Int("count", len(files)),
		)
	}

	return nil
}

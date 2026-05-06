// internal_test.go: white-box tests for catalogue internals.
// Uses package catalogue (not catalogue_test) to access exported-for-test helpers.
package catalogue_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
)

func TestExecMigration_BadDDL(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)

	// Pass deliberately invalid SQL to trigger the exec error path in execMigration.
	err := s.ExecMigrationForTest(context.Background(), "THIS IS NOT VALID SQL !!!")
	assert.Error(t, err, "invalid DDL must return an error")
}

func TestNewStoreForTest_OperatesOnProvidedDB(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	s := catalogue.NewStoreForTest(db)

	// Run migrations so tables exist.
	upDDL := `
		CREATE TABLE IF NOT EXISTS poll_checkpoint (
			source_id TEXT NOT NULL, feed_url TEXT NOT NULL,
			last_polled_at TEXT NOT NULL, last_etag TEXT NOT NULL DEFAULT '',
			last_modified TEXT NOT NULL DEFAULT '', last_status INTEGER NOT NULL DEFAULT 0,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (source_id, feed_url)
		);
		CREATE TABLE IF NOT EXISTS alert_catalogue (
			source_id TEXT NOT NULL, alert_id TEXT NOT NULL,
			last_seen_at TEXT NOT NULL, is_active INTEGER NOT NULL DEFAULT 1,
			content_hash TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (source_id, alert_id)
		);`

	require.NoError(t, s.ExecMigrationForTest(context.Background(), upDDL))

	// Store must be functional after manual migration.
	require.NoError(t, s.Close())
}

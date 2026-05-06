// export_test.go exposes internal functions for white-box testing.
// This file is compiled only when running tests (package catalogue, not catalogue_test).
package catalogue

import (
	"context"
	"database/sql"
)

// ExecMigrationForTest calls the private execMigration method so tests can
// drive the bad-DDL error path without embedding a broken migration file.
func (s *Store) ExecMigrationForTest(ctx context.Context, ddl string) error {
	return s.execMigration(ctx, ddl)
}

// NewStoreForTest wraps an existing *sql.DB in a Store for white-box testing.
// Migrations must already have been applied (use Open or ExecMigrationForTest).
func NewStoreForTest(db *sql.DB) *Store {
	return &Store{db: db}
}

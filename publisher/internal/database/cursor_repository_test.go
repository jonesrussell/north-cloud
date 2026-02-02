package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
)

func TestCursorRepository_GetCursor(t *testing.T) {
	t.Helper()
	runGetCursorTests(t)
}

func runGetCursorTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := database.NewRepository(sqlxDB)
	ctx := context.Background()

	testCases := []struct {
		name       string
		setupMock  func()
		wantCursor []any
		wantErr    bool
	}{
		{
			name: "returns cursor when exists",
			setupMock: func() {
				cursorJSON, _ := json.Marshal([]any{"2026-02-02T12:00:00Z", "doc123"})
				rows := sqlmock.NewRows([]string{"last_sort"}).AddRow(cursorJSON)
				mock.ExpectQuery("SELECT last_sort FROM publisher_cursor").
					WillReturnRows(rows)
			},
			wantCursor: []any{"2026-02-02T12:00:00Z", "doc123"},
			wantErr:    false,
		},
		{
			name: "returns empty cursor when not found",
			setupMock: func() {
				mock.ExpectQuery("SELECT last_sort FROM publisher_cursor").
					WillReturnError(sql.ErrNoRows)
			},
			wantCursor: []any{},
			wantErr:    false,
		},
		{
			name: "returns error on database failure",
			setupMock: func() {
				mock.ExpectQuery("SELECT last_sort FROM publisher_cursor").
					WillReturnError(sql.ErrConnDone)
			},
			wantCursor: nil,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			cursor, callErr := repo.GetCursor(ctx)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("GetCursor() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if !tc.wantErr && len(cursor) != len(tc.wantCursor) {
				t.Errorf("GetCursor() returned %d elements, want %d", len(cursor), len(tc.wantCursor))
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestCursorRepository_UpdateCursor(t *testing.T) {
	t.Helper()
	runUpdateCursorTests(t)
}

func runUpdateCursorTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	repo := database.NewRepository(sqlxDB)
	ctx := context.Background()
	cursor := []any{"2026-02-02T12:00:00Z", "doc123"}

	testCases := []struct {
		name      string
		setupMock func()
		wantErr   bool
	}{
		{
			name: "successfully updates cursor",
			setupMock: func() {
				mock.ExpectExec("INSERT INTO publisher_cursor").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "returns error on database failure",
			setupMock: func() {
				mock.ExpectExec("INSERT INTO publisher_cursor").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			callErr := repo.UpdateCursor(ctx, cursor)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("UpdateCursor() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

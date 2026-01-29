package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/domain"
)

func TestOutboxRepository_MarkPublished(t *testing.T) {
	t.Helper()
	runMarkPublishedTests(t)
}

func runMarkPublishedTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()
	entryID := "test-entry-123"

	testCases := []struct {
		name      string
		setupMock func()
		wantErr   bool
	}{
		{
			name: "successfully marks entry as published",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(entryID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "entry not found returns error",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(entryID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
		},
		{
			name: "database error returns error",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(entryID).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			callErr := repo.MarkPublished(ctx, entryID)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("MarkPublished() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestOutboxRepository_MarkFailed(t *testing.T) {
	t.Helper()
	runMarkFailedTests(t)
}

func runMarkFailedTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()
	entryID := "test-entry-456"
	errorMsg := "Redis connection timeout"

	testCases := []struct {
		name      string
		setupMock func()
		wantErr   bool
	}{
		{
			name: "successfully marks entry as failed",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(entryID, errorMsg).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error returns error",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(entryID, errorMsg).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			callErr := repo.MarkFailed(ctx, entryID, errorMsg)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("MarkFailed() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestOutboxRepository_GetStats(t *testing.T) {
	t.Helper()
	runGetStatsTests(t)
}

func runGetStatsTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()

	testCases := []struct {
		name      string
		setupMock func()
		wantStats *domain.OutboxStats
		wantErr   bool
	}{
		{
			name: "returns correct stats",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{
					"pending", "publishing", "published",
					"failed_retryable", "failed_exhausted", "avg_publish_lag_seconds",
				}).AddRow(10, 2, 100, 3, 1, 5.5)
				mock.ExpectQuery("SELECT").WillReturnRows(rows)
			},
			wantStats: &domain.OutboxStats{
				Pending:         10,
				Publishing:      2,
				Published:       100,
				FailedRetryable: 3,
				FailedExhausted: 1,
			},
			wantErr: false,
		},
		{
			name: "returns empty stats when no entries",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{
					"pending", "publishing", "published",
					"failed_retryable", "failed_exhausted", "avg_publish_lag_seconds",
				}).AddRow(0, 0, 0, 0, 0, 0.0)
				mock.ExpectQuery("SELECT").WillReturnRows(rows)
			},
			wantStats: &domain.OutboxStats{
				Pending:         0,
				Publishing:      0,
				Published:       0,
				FailedRetryable: 0,
				FailedExhausted: 0,
			},
			wantErr: false,
		},
		{
			name: "database error returns error",
			setupMock: func() {
				mock.ExpectQuery("SELECT").WillReturnError(sql.ErrConnDone)
			},
			wantStats: nil,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			stats, callErr := repo.GetStats(ctx)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("GetStats() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			verifyOutboxStats(t, stats, tc.wantStats)

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func verifyOutboxStats(t *testing.T, got, want *domain.OutboxStats) {
	t.Helper()
	if want == nil || got == nil {
		return
	}
	if got.Pending != want.Pending {
		t.Errorf("Pending = %d, want %d", got.Pending, want.Pending)
	}
	if got.Published != want.Published {
		t.Errorf("Published = %d, want %d", got.Published, want.Published)
	}
	if got.FailedRetryable != want.FailedRetryable {
		t.Errorf("FailedRetryable = %d, want %d", got.FailedRetryable, want.FailedRetryable)
	}
}

func TestOutboxRepository_Count(t *testing.T) {
	t.Helper()
	runCountTests(t)
}

func runCountTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()

	testCases := []struct {
		name      string
		status    domain.OutboxStatus
		setupMock func()
		wantCount int64
		wantErr   bool
	}{
		{
			name:   "count pending entries",
			status: domain.OutboxStatusPending,
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(25)
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(domain.OutboxStatusPending).
					WillReturnRows(rows)
			},
			wantCount: 25,
			wantErr:   false,
		},
		{
			name:   "count published entries",
			status: domain.OutboxStatusPublished,
			setupMock: func() {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(100)
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(domain.OutboxStatusPublished).
					WillReturnRows(rows)
			},
			wantCount: 100,
			wantErr:   false,
		},
		{
			name:   "database error returns error",
			status: domain.OutboxStatusPending,
			setupMock: func() {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(domain.OutboxStatusPending).
					WillReturnError(sql.ErrConnDone)
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			count, callErr := repo.Count(ctx, tc.status)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("Count() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if count != tc.wantCount {
				t.Errorf("Count() = %d, want %d", count, tc.wantCount)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestOutboxRepository_ResetToPending(t *testing.T) {
	t.Helper()
	runResetToPendingTests(t)
}

func runResetToPendingTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()
	olderThan := 5 * time.Minute

	testCases := []struct {
		name      string
		setupMock func()
		wantReset int64
		wantErr   bool
	}{
		{
			name: "successfully resets stale entries",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(olderThan.String()).
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			wantReset: 3,
			wantErr:   false,
		},
		{
			name: "no entries to reset",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(olderThan.String()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantReset: 0,
			wantErr:   false,
		},
		{
			name: "database error returns error",
			setupMock: func() {
				mock.ExpectExec("UPDATE classified_outbox").
					WithArgs(olderThan.String()).
					WillReturnError(sql.ErrConnDone)
			},
			wantReset: 0,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			reset, callErr := repo.ResetToPending(ctx, olderThan)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("ResetToPending() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if reset != tc.wantReset {
				t.Errorf("ResetToPending() = %d, want %d", reset, tc.wantReset)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestOutboxRepository_CleanupPublished(t *testing.T) {
	t.Helper()
	runCleanupPublishedTests(t)
}

func runCleanupPublishedTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()
	olderThan := 7 * 24 * time.Hour // 7 days

	testCases := []struct {
		name        string
		setupMock   func()
		wantDeleted int64
		wantErr     bool
	}{
		{
			name: "successfully cleans up old entries",
			setupMock: func() {
				mock.ExpectExec("DELETE FROM classified_outbox").
					WithArgs(olderThan.String()).
					WillReturnResult(sqlmock.NewResult(0, 50))
			},
			wantDeleted: 50,
			wantErr:     false,
		},
		{
			name: "no entries to cleanup",
			setupMock: func() {
				mock.ExpectExec("DELETE FROM classified_outbox").
					WithArgs(olderThan.String()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantDeleted: 0,
			wantErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			deleted, callErr := repo.CleanupPublished(ctx, olderThan)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("CleanupPublished() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if deleted != tc.wantDeleted {
				t.Errorf("CleanupPublished() = %d, want %d", deleted, tc.wantDeleted)
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

func TestOutboxRepository_GetByID(t *testing.T) {
	t.Helper()
	runGetByIDTests(t)
}

func runGetByIDTests(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := database.NewOutboxRepository(db)
	ctx := context.Background()
	entryID := "test-entry-789"
	now := time.Now()

	testCases := []struct {
		name      string
		setupMock func()
		wantEntry bool
		wantErr   bool
	}{
		{
			name: "successfully gets entry by ID",
			setupMock: func() {
				rows := sqlmock.NewRows([]string{
					"id", "content_id", "source_name", "index_name", "content_type", "topics",
					"quality_score", "is_crime_related", "crime_subcategory",
					"title", "body", "url", "published_date", "status", "retry_count",
					"max_retries", "error_message", "created_at", "updated_at",
					"published_at", "next_retry_at",
				}).AddRow(
					entryID, "content-123", "news-source", "news_classified_content", "article",
					pq.StringArray{"news", "local"},
					85, false, nil,
					"Test Article", "Article body", "https://example.com/article", now,
					domain.OutboxStatusPending, 0, 5, nil, now, now, nil, nil,
				)
				mock.ExpectQuery("SELECT").
					WithArgs(entryID).
					WillReturnRows(rows)
			},
			wantEntry: true,
			wantErr:   false,
		},
		{
			name: "entry not found returns ErrNotFound",
			setupMock: func() {
				mock.ExpectQuery("SELECT").
					WithArgs(entryID).
					WillReturnError(sql.ErrNoRows)
			},
			wantEntry: false,
			wantErr:   true, // ErrNotFound is still an error
		},
		{
			name: "database error returns error",
			setupMock: func() {
				mock.ExpectQuery("SELECT").
					WithArgs(entryID).
					WillReturnError(sql.ErrConnDone)
			},
			wantEntry: false,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()

			entry, callErr := repo.GetByID(ctx, entryID)
			if (callErr != nil) != tc.wantErr {
				t.Errorf("GetByID() error = %v, wantErr %v", callErr, tc.wantErr)
			}

			if tc.wantEntry && entry == nil {
				t.Error("GetByID() returned nil entry, want non-nil")
			}

			if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
				t.Errorf("unfulfilled expectations: %v", expectErr)
			}
		})
	}
}

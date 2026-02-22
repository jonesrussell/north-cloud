package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

// feedStateColumns lists the columns returned by feed_state SELECT queries.
var feedStateColumns = []string{
	"source_id", "feed_url", "last_polled_at", "last_etag", "last_modified",
	"last_item_count", "consecutive_errors", "last_error", "last_error_type",
	"created_at", "updated_at",
}

func newFeedStateRepo(t *testing.T) (*database.FeedStateRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewFeedStateRepository(db)

	return repo, mock, func() { mockDB.Close() }
}

func TestFeedStateRepository_GetOrCreate_NewSource(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("INSERT INTO feed_state").
		WithArgs("source-uuid-1", "https://example.com/rss").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM feed_state WHERE source_id").
		WithArgs("source-uuid-1").
		WillReturnRows(
			sqlmock.NewRows(feedStateColumns).AddRow(
				"source-uuid-1", "https://example.com/rss",
				nil, nil, nil, 0, 0, nil, nil, now, now,
			),
		)

	state, err := repo.GetOrCreate(ctx, "source-uuid-1", "https://example.com/rss")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if state.SourceID != "source-uuid-1" {
		t.Errorf("expected SourceID=source-uuid-1, got %s", state.SourceID)
	}
	if state.FeedURL != "https://example.com/rss" {
		t.Errorf("expected FeedURL=https://example.com/rss, got %s", state.FeedURL)
	}
	if state.LastPolledAt != nil {
		t.Errorf("expected LastPolledAt=nil, got %v", state.LastPolledAt)
	}
	if state.ConsecutiveErrors != 0 {
		t.Errorf("expected ConsecutiveErrors=0, got %d", state.ConsecutiveErrors)
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_GetOrCreate_ExistingSource(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	polledAt := now.Add(-time.Hour)
	etag := `"abc123"`
	modified := "Wed, 15 Feb 2026 12:00:00 GMT"
	itemCount := 25

	// INSERT does nothing on conflict (existing source)
	mock.ExpectExec("INSERT INTO feed_state").
		WithArgs("source-uuid-1", "https://example.com/rss").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT .+ FROM feed_state WHERE source_id").
		WithArgs("source-uuid-1").
		WillReturnRows(
			sqlmock.NewRows(feedStateColumns).AddRow(
				"source-uuid-1", "https://example.com/rss",
				polledAt, etag, modified, itemCount, 0, nil, nil, now, now,
			),
		)

	state, err := repo.GetOrCreate(ctx, "source-uuid-1", "https://example.com/rss")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if state.LastPolledAt == nil {
		t.Fatal("expected LastPolledAt to be non-nil")
	}
	if state.LastItemCount != itemCount {
		t.Errorf("expected LastItemCount=%d, got %d", itemCount, state.LastItemCount)
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_GetOrCreate_InsertError(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("INSERT INTO feed_state").
		WithArgs("source-uuid-1", "https://example.com/rss").
		WillReturnError(errors.New("connection refused"))

	_, err := repo.GetOrCreate(ctx, "source-uuid-1", "https://example.com/rss")
	if err == nil {
		t.Fatal("GetOrCreate() expected error, got nil")
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_UpdateSuccess(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	etag := `"etag-value"`
	modified := "Wed, 15 Feb 2026 12:00:00 GMT"
	itemCount := 42

	mock.ExpectExec("UPDATE feed_state").
		WithArgs("source-uuid-1", &etag, &modified, itemCount).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateSuccess(ctx, "source-uuid-1", database.FeedPollResult{
		ETag:      &etag,
		Modified:  &modified,
		ItemCount: itemCount,
	})
	if err != nil {
		t.Fatalf("UpdateSuccess() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_UpdateSuccess_NotFound(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE feed_state").
		WithArgs("nonexistent-id", nil, nil, 0).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateSuccess(ctx, "nonexistent-id", database.FeedPollResult{})
	if err == nil {
		t.Fatal("UpdateSuccess() expected error for non-existent source, got nil")
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_UpdateError(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE feed_state").
		WithArgs("source-uuid-1", "parse_error", "feed parse error: invalid XML").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateError(ctx, "source-uuid-1", "parse_error", "feed parse error: invalid XML")
	if err != nil {
		t.Fatalf("UpdateError() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_UpdateError_NotFound(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE feed_state").
		WithArgs("nonexistent-id", "unexpected", "some error").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateError(ctx, "nonexistent-id", "unexpected", "some error")
	if err == nil {
		t.Fatal("UpdateError() expected error for non-existent source, got nil")
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_ListDueForPolling(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	defaultInterval := 30

	mock.ExpectQuery("SELECT .+ FROM feed_state\\s+WHERE last_polled_at IS NULL").
		WithArgs(defaultInterval).
		WillReturnRows(
			sqlmock.NewRows(feedStateColumns).
				AddRow("source-never-polled", "https://a.com/rss", nil, nil, nil, 0, 0, nil, nil, now, now).
				AddRow("source-old-poll", "https://b.com/rss", now.Add(-time.Hour), nil, nil, 10, 0, nil, nil, now, now),
		)

	states, err := repo.ListDueForPolling(ctx, defaultInterval)
	if err != nil {
		t.Fatalf("ListDueForPolling() error = %v", err)
	}

	expectedCount := 2
	if len(states) != expectedCount {
		t.Fatalf("expected %d feed states, got %d", expectedCount, len(states))
	}

	// First result should be the never-polled feed (NULLS FIRST)
	if states[0].SourceID != "source-never-polled" {
		t.Errorf("expected first result source_id=source-never-polled, got %s", states[0].SourceID)
	}
	if states[1].SourceID != "source-old-poll" {
		t.Errorf("expected second result source_id=source-old-poll, got %s", states[1].SourceID)
	}

	expectationsMet(t, mock)
}

func TestFeedStateRepository_ListDueForPolling_Empty(t *testing.T) {
	repo, mock, cleanup := newFeedStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	defaultInterval := 30

	mock.ExpectQuery("SELECT .+ FROM feed_state\\s+WHERE last_polled_at IS NULL").
		WithArgs(defaultInterval).
		WillReturnRows(sqlmock.NewRows(feedStateColumns))

	states, err := repo.ListDueForPolling(ctx, defaultInterval)
	if err != nil {
		t.Fatalf("ListDueForPolling() error = %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 feed states, got %d", len(states))
	}

	expectationsMet(t, mock)
}

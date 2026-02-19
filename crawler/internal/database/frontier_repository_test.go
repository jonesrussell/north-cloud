package database_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
)

// frontierColumns lists the columns returned by frontier SELECT queries.
var frontierColumns = []string{
	"id", "url", "url_hash", "host", "source_id", "origin", "parent_url", "depth",
	"priority", "status", "next_fetch_at", "last_fetched_at", "fetch_count",
	"content_hash", "etag", "last_modified", "retry_count", "last_error",
	"discovered_at", "created_at", "updated_at",
}

func newFrontierRepo(t *testing.T) (*database.FrontierRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewFrontierRepository(db)

	return repo, mock, func() { mockDB.Close() }
}

func expectationsMet(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestFrontierRepository_Submit_NewURL(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("INSERT INTO url_frontier").
		WithArgs(
			"https://example.com/page1",
			"abc123hash",
			"example.com",
			"source-uuid-1",
			"feed",
			nil,
			0,
			5,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Submit(ctx, database.SubmitParams{
		URL:      "https://example.com/page1",
		URLHash:  "abc123hash",
		Host:     "example.com",
		SourceID: "source-uuid-1",
		Origin:   "feed",
		Depth:    0,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_Submit_DuplicateUpdatesPriority(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	// First insert succeeds
	mock.ExpectExec("INSERT INTO url_frontier").
		WithArgs(
			"https://example.com/page1",
			"abc123hash",
			"example.com",
			"source-uuid-1",
			"feed",
			nil,
			0,
			5,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Submit(ctx, database.SubmitParams{
		URL:      "https://example.com/page1",
		URLHash:  "abc123hash",
		Host:     "example.com",
		SourceID: "source-uuid-1",
		Origin:   "feed",
		Depth:    0,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("Submit() first call error = %v", err)
	}

	// Second insert with higher priority triggers ON CONFLICT DO UPDATE
	mock.ExpectExec("INSERT INTO url_frontier").
		WithArgs(
			"https://example.com/page1",
			"abc123hash",
			"example.com",
			"source-uuid-1",
			"spider",
			nil,
			1,
			8,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Submit(ctx, database.SubmitParams{
		URL:      "https://example.com/page1",
		URLHash:  "abc123hash",
		Host:     "example.com",
		SourceID: "source-uuid-1",
		Origin:   "spider",
		Depth:    1,
		Priority: 8,
	})
	if err != nil {
		t.Fatalf("Submit() second call error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_Claim_ReturnsHighestPriority(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .+ FROM url_frontier f").
		WillReturnRows(
			sqlmock.NewRows(frontierColumns).AddRow(
				"url-id-1",
				"https://example.com",
				"hashABC",
				"example.com",
				"source-1",
				"feed",
				nil,
				0,
				10,
				"pending",
				now,
				nil,
				0,
				nil,
				nil,
				nil,
				0,
				nil,
				now,
				now,
				now,
			),
		)
	mock.ExpectExec("UPDATE url_frontier SET status = 'fetching'").
		WithArgs("url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	url, err := repo.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if url == nil {
		t.Fatal("Claim() returned nil, expected a URL")
	}
	if url.ID != "url-id-1" {
		t.Errorf("expected ID=url-id-1, got %s", url.ID)
	}
	if url.Status != "fetching" {
		t.Errorf("expected status=fetching, got %s", url.Status)
	}
	if url.Priority != 10 {
		t.Errorf("expected priority=10, got %d", url.Priority)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_Claim_ReturnsErrWhenEmpty(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .+ FROM url_frontier f").
		WillReturnRows(sqlmock.NewRows(frontierColumns))
	mock.ExpectRollback()

	url, err := repo.Claim(ctx)
	if !errors.Is(err, database.ErrNoURLAvailable) {
		t.Fatalf("Claim() expected ErrNoURLAvailable, got %v", err)
	}
	if url != nil {
		t.Errorf("Claim() returned %v, expected nil", url)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateFetched(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	contentHash := "sha256:abc123"
	etag := `"etag-value"`
	lastModified := "Wed, 15 Feb 2026 12:00:00 GMT"

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs(&contentHash, &etag, &lastModified, "url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateFetched(ctx, "url-id-1", database.FetchedParams{
		ContentHash:  &contentHash,
		ETag:         &etag,
		LastModified: &lastModified,
	})
	if err != nil {
		t.Fatalf("UpdateFetched() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateFailed_RetriesWhenUnderMax(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	maxRetries := 3

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("connection timeout", maxRetries, "url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateFailed(ctx, "url-id-1", "connection timeout", maxRetries)
	if err != nil {
		t.Fatalf("UpdateFailed() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateFailed_MarksDeadAtMaxRetries(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	maxRetries := 3

	// The SQL uses retry_count + 1 >= maxRetries to decide dead vs pending.
	// We just verify the query executes with correct args.
	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("permanent failure", maxRetries, "url-id-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateFailed(ctx, "url-id-2", "permanent failure", maxRetries)
	if err != nil {
		t.Fatalf("UpdateFailed() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateDead(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("robots.txt disallowed", "url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateDead(ctx, "url-id-1", "robots.txt disallowed")
	if err != nil {
		t.Fatalf("UpdateDead() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateDead_NotFound(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("reason", "nonexistent-id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateDead(ctx, "nonexistent-id", "reason")
	if err == nil {
		t.Fatal("UpdateDead() expected error for non-existent URL, got nil")
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateFetchedWithFinalURL_Success(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	finalURL := "https://final.example.com/page"
	contentHash := "abc123"
	params := database.FetchedParams{ContentHash: &contentHash}

	normalized, err := frontier.NormalizeURL(finalURL)
	if err != nil {
		t.Fatalf("NormalizeURL: %v", err)
	}
	urlHash, err := frontier.URLHash(finalURL)
	if err != nil {
		t.Fatalf("URLHash: %v", err)
	}
	host, err := frontier.ExtractHost(finalURL)
	if err != nil {
		t.Fatalf("ExtractHost: %v", err)
	}

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs(normalized, urlHash, host, &contentHash, nil, nil, "url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateFetchedWithFinalURL(ctx, "url-id-1", finalURL, params)
	if err != nil {
		t.Fatalf("UpdateFetchedWithFinalURL() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_UpdateFetchedWithFinalURL_UniqueViolationFallback(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	finalURL := "https://other.example.com/article"
	params := database.FetchedParams{}

	normalized, err := frontier.NormalizeURL(finalURL)
	if err != nil {
		t.Fatalf("NormalizeURL: %v", err)
	}
	urlHash, err := frontier.URLHash(finalURL)
	if err != nil {
		t.Fatalf("URLHash: %v", err)
	}
	host, err := frontier.ExtractHost(finalURL)
	if err != nil {
		t.Fatalf("ExtractHost: %v", err)
	}

	// First UPDATE hits unique constraint 23505.
	mock.ExpectExec("UPDATE url_frontier").
		WithArgs(normalized, urlHash, host, nil, nil, nil, "url-id-1").
		WillReturnError(&pq.Error{Code: "23505"})

	// Fallback: UpdateFetched (no url/url_hash/host in this UPDATE).
	mock.ExpectExec("UPDATE url_frontier").
		WithArgs(nil, nil, nil, "url-id-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateFetchedWithFinalURL(ctx, "url-id-1", finalURL, params)
	if err != nil {
		t.Fatalf("UpdateFetchedWithFinalURL() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_ResetForRetry(t *testing.T) {
	t.Helper()

	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("dead-url-id").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.ResetForRetry(ctx, "dead-url-id")
	if err != nil {
		t.Fatalf("ResetForRetry() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_ResetForRetry_NotFoundOrNotDead(t *testing.T) {
	t.Helper()

	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE url_frontier").
		WithArgs("pending-url-id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.ResetForRetry(ctx, "pending-url-id")
	if err == nil {
		t.Fatal("ResetForRetry() expected error when URL is not dead, got nil")
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_List_WithFilters(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Expect count query
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM url_frontier WHERE status = \\$1 AND source_id = \\$2").
		WithArgs("pending", "source-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Expect select query
	mock.ExpectQuery("SELECT .+ FROM url_frontier f\\s+WHERE status = \\$1 AND source_id = \\$2").
		WithArgs("pending", "source-1", 50, 0).
		WillReturnRows(
			sqlmock.NewRows(frontierColumns).AddRow(
				"url-id-1", "https://example.com", "hash1", "example.com", "source-1",
				"feed", nil, 0, 5, "pending", now, nil, 0, nil, nil, nil, 0, nil, now, now, now,
			),
		)

	urls, count, err := repo.List(ctx, database.FrontierFilters{
		Status:   "pending",
		SourceID: "source-1",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if len(urls) != 1 {
		t.Errorf("expected 1 URL, got %d", len(urls))
	}

	expectationsMet(t, mock)
}

func TestFrontierRepository_Stats(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery("SELECT status, COUNT\\(\\*\\) FROM url_frontier GROUP BY status").
		WillReturnRows(
			sqlmock.NewRows([]string{"status", "count"}).
				AddRow("pending", 100).
				AddRow("fetching", 5).
				AddRow("fetched", 500).
				AddRow("failed", 10).
				AddRow("dead", 3),
		)

	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	assertStatCount(t, "TotalPending", stats.TotalPending, 100)
	assertStatCount(t, "TotalFetching", stats.TotalFetching, 5)
	assertStatCount(t, "TotalFetched", stats.TotalFetched, 500)
	assertStatCount(t, "TotalFailed", stats.TotalFailed, 10)
	assertStatCount(t, "TotalDead", stats.TotalDead, 3)

	expectationsMet(t, mock)
}

func TestFrontierRepository_Stats_EmptyFrontier(t *testing.T) {
	repo, mock, cleanup := newFrontierRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery("SELECT status, COUNT\\(\\*\\) FROM url_frontier GROUP BY status").
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}))

	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	assertStatCount(t, "TotalPending", stats.TotalPending, 0)
	assertStatCount(t, "TotalFetching", stats.TotalFetching, 0)
	assertStatCount(t, "TotalFetched", stats.TotalFetched, 0)
	assertStatCount(t, "TotalFailed", stats.TotalFailed, 0)
	assertStatCount(t, "TotalDead", stats.TotalDead, 0)

	expectationsMet(t, mock)
}

func assertStatCount(t *testing.T, field string, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%d, got %d", field, want, got)
	}
}

// Verify driver.Value interface compliance for nil *string args.
var _ driver.Value = (*string)(nil)

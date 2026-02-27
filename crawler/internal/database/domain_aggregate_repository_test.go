package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

// linksCols is the number of columns returned by the discovered_links query.
const linksCols = 15

func newAggregateRepo(t *testing.T) (*database.DomainAggregateRepository, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := sqlx.NewDb(mockDB, "postgres")

	t.Cleanup(func() { mockDB.Close() })

	return database.NewDomainAggregateRepository(db), mock
}

func TestDomainAggregateRepository_ListAggregates(t *testing.T) {
	t.Parallel()

	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	okRatio := 0.95
	htmlRatio := 0.80

	cols := []string{
		"domain", "status", "link_count", "source_count",
		"avg_depth", "first_seen", "last_seen",
		"ok_ratio", "html_ratio", "notes",
	}

	rows := sqlmock.NewRows(cols).
		AddRow("news.com", "active", 42, 3, 1.5, now, now, okRatio, htmlRatio, nil)

	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	filters := database.DomainListFilters{
		Limit: 25,
	}

	results, err := repo.ListAggregates(ctx, filters)
	if err != nil {
		t.Fatalf("ListAggregates() error = %v", err)
	}

	expectedLen := 1
	if len(results) != expectedLen {
		t.Fatalf("expected %d result(s), got %d", expectedLen, len(results))
	}

	if results[0].Domain != "news.com" {
		t.Errorf("expected domain news.com, got %s", results[0].Domain)
	}

	expectedLinkCount := 42
	if results[0].LinkCount != expectedLinkCount {
		t.Errorf("expected link_count=%d, got %d", expectedLinkCount, results[0].LinkCount)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_CountAggregates(t *testing.T) {
	t.Parallel()

	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	filters := database.DomainListFilters{}

	count, err := repo.CountAggregates(ctx, filters)
	if err != nil {
		t.Fatalf("CountAggregates() error = %v", err)
	}

	expectedCount := 15
	if count != expectedCount {
		t.Errorf("expected count=%d, got %d", expectedCount, count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_GetReferringSources(t *testing.T) {
	t.Parallel()

	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"source_name"}).
		AddRow("Source A").
		AddRow("Source B")

	mock.ExpectQuery("SELECT DISTINCT source_name").
		WithArgs("news.com").
		WillReturnRows(rows)

	sources, err := repo.GetReferringSources(ctx, "news.com")
	if err != nil {
		t.Fatalf("GetReferringSources() error = %v", err)
	}

	expectedSourceCount := 2
	if len(sources) != expectedSourceCount {
		t.Fatalf("expected %d sources, got %d", expectedSourceCount, len(sources))
	}

	if sources[0] != "Source A" {
		t.Errorf("expected first source 'Source A', got %q", sources[0])
	}

	if sources[1] != "Source B" {
		t.Errorf("expected second source 'Source B', got %q", sources[1])
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_ListLinksByDomain(t *testing.T) {
	t.Parallel()

	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	httpStatus := int16(200)
	contentType := "text/html"
	parentURL := "https://news.com/"

	cols := []string{
		"id", "source_id", "source_name", "url", "parent_url",
		"depth", "domain", "http_status", "content_type", "discovered_at",
		"queued_at", "status", "priority", "created_at", "updated_at",
	}

	if len(cols) != linksCols {
		t.Fatalf("expected %d columns, got %d", linksCols, len(cols))
	}

	rows := sqlmock.NewRows(cols).
		AddRow(
			"link-1", "src-1", "Source A",
			"https://news.com/article", &parentURL,
			1, "news.com", &httpStatus, &contentType, now,
			now, "discovered", 0, now, now,
		)

	mock.ExpectQuery("SELECT .+ FROM discovered_links WHERE domain").
		WithArgs("news.com", 25, 0).
		WillReturnRows(rows)

	// Expect count query
	expectedTotal := 50

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("news.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(expectedTotal))

	links, total, err := repo.ListLinksByDomain(ctx, "news.com", 25, 0)
	if err != nil {
		t.Fatalf("ListLinksByDomain() error = %v", err)
	}

	expectedLinkLen := 1
	if len(links) != expectedLinkLen {
		t.Fatalf("expected %d link(s), got %d", expectedLinkLen, len(links))
	}

	if links[0].URL != "https://news.com/article" {
		t.Errorf("expected URL https://news.com/article, got %s", links[0].URL)
	}

	if total != expectedTotal {
		t.Errorf("expected total=%d, got %d", expectedTotal, total)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

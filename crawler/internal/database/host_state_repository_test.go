package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

// hostStateColumns lists the columns returned by host_state SELECT queries.
var hostStateColumns = []string{
	"host", "last_fetch_at", "min_delay_ms", "robots_txt",
	"robots_fetched_at", "robots_ttl_hours", "created_at", "updated_at",
}

// defaultMinDelayMs is the default min_delay_ms value from the schema.
const defaultMinDelayMs = 1000

// defaultRobotsTTLHours is the default robots_ttl_hours value from the schema.
const defaultRobotsTTLHours = 24

func newHostStateRepo(t *testing.T) (*database.HostStateRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewHostStateRepository(db)

	return repo, mock, func() { mockDB.Close() }
}

func TestHostState_GetOrCreate_NewHost(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec("INSERT INTO host_state").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM host_state WHERE host").
		WithArgs("example.com").
		WillReturnRows(
			sqlmock.NewRows(hostStateColumns).AddRow(
				"example.com", nil, defaultMinDelayMs, nil,
				nil, defaultRobotsTTLHours, now, now,
			),
		)

	state, err := repo.GetOrCreate(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if state.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", state.Host)
	}
	if state.MinDelayMs != defaultMinDelayMs {
		t.Errorf("expected min_delay_ms=%d, got %d", defaultMinDelayMs, state.MinDelayMs)
	}
	if state.LastFetchAt != nil {
		t.Errorf("expected last_fetch_at=nil, got %v", state.LastFetchAt)
	}
	if state.RobotsTxt != nil {
		t.Errorf("expected robots_txt=nil, got %v", state.RobotsTxt)
	}

	expectationsMet(t, mock)
}

func TestHostState_GetOrCreate_ExistingHost(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	robotsTxt := "User-agent: *\nDisallow: /admin"

	// INSERT is a no-op for existing host (ON CONFLICT DO NOTHING).
	mock.ExpectExec("INSERT INTO host_state").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("SELECT .+ FROM host_state WHERE host").
		WithArgs("example.com").
		WillReturnRows(
			sqlmock.NewRows(hostStateColumns).AddRow(
				"example.com", &now, defaultMinDelayMs, &robotsTxt,
				&now, defaultRobotsTTLHours, now, now,
			),
		)

	state, err := repo.GetOrCreate(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if state.Host != "example.com" {
		t.Errorf("expected host=example.com, got %s", state.Host)
	}
	if state.LastFetchAt == nil {
		t.Error("expected last_fetch_at to be set, got nil")
	}
	if state.RobotsTxt == nil || *state.RobotsTxt != robotsTxt {
		t.Errorf("expected robots_txt=%q, got %v", robotsTxt, state.RobotsTxt)
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateLastFetch(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE host_state SET last_fetch_at").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateLastFetch(ctx, "example.com")
	if err != nil {
		t.Fatalf("UpdateLastFetch() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateLastFetch_NotFound(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE host_state SET last_fetch_at").
		WithArgs("unknown.com").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateLastFetch(ctx, "unknown.com")
	if err == nil {
		t.Fatal("UpdateLastFetch() expected error for non-existent host, got nil")
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateRobotsTxt_WithCrawlDelay(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	crawlDelay := 2000

	mock.ExpectExec("UPDATE host_state").
		WithArgs("example.com", "User-agent: *\nDisallow: /", &crawlDelay).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateRobotsTxt(ctx, "example.com", "User-agent: *\nDisallow: /", &crawlDelay)
	if err != nil {
		t.Fatalf("UpdateRobotsTxt() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateRobotsTxt_NilCrawlDelay(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec("UPDATE host_state").
		WithArgs("example.com", "User-agent: *\nAllow: /", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateRobotsTxt(ctx, "example.com", "User-agent: *\nAllow: /", nil)
	if err != nil {
		t.Fatalf("UpdateRobotsTxt() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateMinDelay(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	newDelay := 5000

	mock.ExpectExec("UPDATE host_state SET min_delay_ms").
		WithArgs("example.com", newDelay).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateMinDelay(ctx, "example.com", newDelay)
	if err != nil {
		t.Fatalf("UpdateMinDelay() error = %v", err)
	}

	expectationsMet(t, mock)
}

func TestHostState_UpdateMinDelay_NotFound(t *testing.T) {
	repo, mock, cleanup := newHostStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	newDelay := 5000

	mock.ExpectExec("UPDATE host_state SET min_delay_ms").
		WithArgs("unknown.com", newDelay).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateMinDelay(ctx, "unknown.com", newDelay)
	if err == nil {
		t.Fatal("UpdateMinDelay() expected error for non-existent host, got nil")
	}

	expectationsMet(t, mock)
}

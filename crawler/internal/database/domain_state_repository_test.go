package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// stateRepoCols enumerates the columns returned by SELECT * from discovered_domain_states.
const stateRepoCols = 9

func newStateRepo(t *testing.T) (*database.DomainStateRepository, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	db := sqlx.NewDb(mockDB, "postgres")

	t.Cleanup(func() { mockDB.Close() })

	return database.NewDomainStateRepository(db), mock
}

func stringPtr(s string) *string {
	return &s
}

func TestDomainStateRepository_Upsert(t *testing.T) {
	t.Parallel()

	repo, mock := newStateRepo(t)
	ctx := context.Background()

	// Expect INSERT ... ON CONFLICT exec for upsert
	mock.ExpectExec("INSERT INTO discovered_domain_states").
		WithArgs("example.com", domain.DomainStatusIgnored, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect UPDATE ignored_at for "ignored" status
	mock.ExpectExec("UPDATE discovered_domain_states SET ignored_at").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Upsert(ctx, "example.com", domain.DomainStatusIgnored, nil)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_GetByDomain_Found(t *testing.T) {
	t.Parallel()

	repo, mock := newStateRepo(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	cols := []string{
		"domain", "status", "notes", "ignored_at", "ignored_by",
		"promoted_at", "promoted_source_id", "created_at", "updated_at",
	}

	if len(cols) != stateRepoCols {
		t.Fatalf("expected %d columns, got %d", stateRepoCols, len(cols))
	}

	rows := sqlmock.NewRows(cols).
		AddRow("example.com", domain.DomainStatusIgnored, "test note", now, "admin", nil, nil, now, now)

	mock.ExpectQuery("SELECT \\* FROM discovered_domain_states WHERE domain").
		WithArgs("example.com").
		WillReturnRows(rows)

	state, err := repo.GetByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetByDomain() error = %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", state.Domain)
	}

	if state.Status != domain.DomainStatusIgnored {
		t.Errorf("expected status %s, got %s", domain.DomainStatusIgnored, state.Status)
	}

	if state.Notes == nil || *state.Notes != "test note" {
		t.Errorf("expected notes 'test note', got %v", state.Notes)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_GetByDomain_NotFound(t *testing.T) {
	t.Parallel()

	repo, mock := newStateRepo(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM discovered_domain_states WHERE domain").
		WithArgs("nonexistent.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"domain", "status", "notes", "ignored_at", "ignored_by",
			"promoted_at", "promoted_source_id", "created_at", "updated_at",
		}))

	state, err := repo.GetByDomain(ctx, "nonexistent.com")
	if err != nil {
		t.Fatalf("GetByDomain() error = %v", err)
	}

	if state != nil {
		t.Errorf("expected nil state for nonexistent domain, got %+v", state)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_BulkUpsert(t *testing.T) {
	t.Parallel()

	repo, mock := newStateRepo(t)
	ctx := context.Background()

	domains := []string{"a.com", "b.com"}
	notes := stringPtr("bulk note")

	// Expect Begin
	mock.ExpectBegin()

	// Expect bulk INSERT exec
	mock.ExpectExec("INSERT INTO discovered_domain_states").
		WithArgs("a.com", domain.DomainStatusIgnored, notes, "b.com", domain.DomainStatusIgnored, notes).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect UPDATE ignored_at for bulk timestamp (rebind uses $1, $2)
	mock.ExpectExec("UPDATE discovered_domain_states SET ignored_at").
		WithArgs("a.com", "b.com").
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect Commit
	mock.ExpectCommit()

	count, err := repo.BulkUpsert(ctx, domains, domain.DomainStatusIgnored, notes)
	if err != nil {
		t.Fatalf("BulkUpsert() error = %v", err)
	}

	expectedCount := 2
	if count != expectedCount {
		t.Errorf("expected count=%d, got %d", expectedCount, count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

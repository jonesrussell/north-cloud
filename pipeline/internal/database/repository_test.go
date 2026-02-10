//nolint:testpackage // Testing internal repository requires same package access
package database

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

func TestRepository_UpsertArticle(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	mock.ExpectExec("INSERT INTO articles").
		WithArgs("https://example.com/article", sqlmock.AnyArg(), "example.com", "example_com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	upsertErr := repo.UpsertArticle(ctx, &domain.Article{
		URL:        "https://example.com/article",
		URLHash:    domain.URLHash("https://example.com/article"),
		Domain:     "example.com",
		SourceName: "example_com",
	})

	if upsertErr != nil {
		t.Errorf("UpsertArticle() error = %v", upsertErr)
	}

	if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
		t.Errorf("unfulfilled expectations: %v", expectErr)
	}
}

func TestRepository_InsertEvent(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	occurredAt := time.Date(2026, 2, 10, 14, 30, 0, 0, time.UTC)

	mock.ExpectQuery("INSERT INTO pipeline_events").
		WithArgs(
			"https://example.com/article",
			"classified",
			occurredAt,
			"classifier",
			sqlmock.AnyArg(), // metadata JSONB
			1,                // schema version
			sqlmock.AnyArg(), // idempotency key
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	event := &domain.PipelineEvent{
		ArticleURL:            "https://example.com/article",
		Stage:                 domain.StageClassified,
		OccurredAt:            occurredAt,
		ServiceName:           "classifier",
		Metadata:              map[string]any{"quality_score": 78},
		MetadataSchemaVersion: 1,
		IdempotencyKey:        "test-key",
	}

	insertErr := repo.InsertEvent(ctx, event)
	if insertErr != nil {
		t.Errorf("InsertEvent() error = %v", insertErr)
	}

	if expectErr := mock.ExpectationsWereMet(); expectErr != nil {
		t.Errorf("unfulfilled expectations: %v", expectErr)
	}
}

func TestRepository_GetFunnel(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	if setupErr != nil {
		t.Fatalf("failed to create sqlmock: %v", setupErr)
	}
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	from := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"stage", "count", "unique_articles"}).
		AddRow("crawled", 100, 90).
		AddRow("indexed", 90, 85).
		AddRow("classified", 80, 70)

	mock.ExpectQuery("SELECT").
		WithArgs(from, to).
		WillReturnRows(rows)

	stages, queryErr := repo.GetFunnel(ctx, from, to)
	if queryErr != nil {
		t.Fatalf("GetFunnel() error = %v", queryErr)
	}

	const expectedStages = 3
	if len(stages) != expectedStages {
		t.Errorf("GetFunnel() returned %d stages, want %d", len(stages), expectedStages)
	}

	if stages[0].Name != "crawled" {
		t.Errorf("stages[0].Name = %q, want %q", stages[0].Name, "crawled")
	}
}

//nolint:testpackage // Testing internal service requires same package access
package service

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type mockRepository struct {
	upsertArticleFunc func(ctx context.Context, article *domain.Article) error
	insertEventFunc   func(ctx context.Context, event *domain.PipelineEvent) error
	getFunnelFunc     func(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error)
}

func (m *mockRepository) UpsertArticle(ctx context.Context, article *domain.Article) error {
	if m.upsertArticleFunc != nil {
		return m.upsertArticleFunc(ctx, article)
	}
	return nil
}

func (m *mockRepository) InsertEvent(ctx context.Context, event *domain.PipelineEvent) error {
	if m.insertEventFunc != nil {
		return m.insertEventFunc(ctx, event)
	}
	return nil
}

func (m *mockRepository) GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error) {
	if m.getFunnelFunc != nil {
		return m.getFunnelFunc(ctx, from, to)
	}
	return nil, nil
}

func (m *mockRepository) Ping(_ context.Context) error { return nil }

type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Info(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Warn(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Error(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Fatal(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) With(_ ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                    { return nil }

func TestPipelineService_Ingest_ValidEvent(t *testing.T) {
	var upsertCalled, insertCalled bool
	repo := &mockRepository{
		upsertArticleFunc: func(_ context.Context, article *domain.Article) error {
			upsertCalled = true
			if article.Domain != "example.com" {
				t.Errorf("article.Domain = %q, want %q", article.Domain, "example.com")
			}
			return nil
		},
		insertEventFunc: func(_ context.Context, event *domain.PipelineEvent) error {
			insertCalled = true
			if event.IdempotencyKey == "" {
				t.Error("expected idempotency key to be auto-generated")
			}
			return nil
		},
	}

	svc := NewPipelineService(repo, &mockLogger{})
	ctx := t.Context()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().UTC(),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr != nil {
		t.Fatalf("Ingest() error = %v", ingestErr)
	}

	if !upsertCalled {
		t.Error("expected UpsertArticle to be called")
	}
	if !insertCalled {
		t.Error("expected InsertEvent to be called")
	}
}

func TestPipelineService_Ingest_RejectsNonUTC(t *testing.T) {
	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := t.Context()

	est, loadErr := time.LoadLocation("America/New_York")
	if loadErr != nil {
		t.Fatalf("failed to load timezone: %v", loadErr)
	}

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().In(est),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for non-UTC timestamp, got nil")
	}
}

func TestPipelineService_Ingest_RejectsInvalidStage(t *testing.T) {
	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := t.Context()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.Stage("invalid"),
		OccurredAt:  time.Now().UTC(),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for invalid stage, got nil")
	}
}

func TestPipelineService_Ingest_RejectsFutureTimestamp(t *testing.T) {
	svc := NewPipelineService(&mockRepository{}, &mockLogger{})
	ctx := t.Context()

	req := &domain.IngestRequest{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       domain.StageCrawled,
		OccurredAt:  time.Now().UTC().Add(time.Hour),
		ServiceName: "crawler",
	}

	ingestErr := svc.Ingest(ctx, req)
	if ingestErr == nil {
		t.Fatal("Ingest() expected error for future timestamp, got nil")
	}
}

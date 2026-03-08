//nolint:testpackage // Testing internal service requires same package access
package service

import (
	"context"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

type mockRepository struct {
	upsertContentItemFunc func(ctx context.Context, item *domain.ContentItem) error
	insertEventFunc       func(ctx context.Context, event *domain.PipelineEvent) error
	getFunnelFunc         func(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error)
}

func (m *mockRepository) UpsertContentItem(ctx context.Context, item *domain.ContentItem) error {
	if m.upsertContentItemFunc != nil {
		return m.upsertContentItemFunc(ctx, item)
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
		upsertContentItemFunc: func(_ context.Context, item *domain.ContentItem) error {
			upsertCalled = true
			if item.Domain != "example.com" {
				t.Errorf("item.Domain = %q, want %q", item.Domain, "example.com")
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
		ContentURL:  "https://example.com/article",
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
		t.Error("expected UpsertContentItem to be called")
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
		ContentURL:  "https://example.com/article",
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
		ContentURL:  "https://example.com/article",
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
		ContentURL:  "https://example.com/article",
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

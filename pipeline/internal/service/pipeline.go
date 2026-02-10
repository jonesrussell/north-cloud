package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const maxEventAge = 24 * time.Hour

// Repository is the data access interface for pipeline events.
type Repository interface {
	UpsertArticle(ctx context.Context, article *domain.Article) error
	InsertEvent(ctx context.Context, event *domain.PipelineEvent) error
	GetFunnel(ctx context.Context, from, to time.Time) ([]domain.FunnelStage, error)
	Ping(ctx context.Context) error
}

// PipelineService handles pipeline event ingestion and querying.
type PipelineService struct {
	repo   Repository
	logger infralogger.Logger
}

// NewPipelineService creates a new pipeline service.
func NewPipelineService(repo Repository, logger infralogger.Logger) *PipelineService {
	return &PipelineService{
		repo:   repo,
		logger: logger,
	}
}

// Ingest validates and stores a single pipeline event.
func (s *PipelineService) Ingest(ctx context.Context, req *domain.IngestRequest) error {
	if validateErr := s.validateIngestRequest(req); validateErr != nil {
		return validateErr
	}

	// Auto-generate idempotency key if not provided
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = domain.GenerateIdempotencyKey(
			req.ServiceName, req.Stage, req.ArticleURL, req.OccurredAt,
		)
	}

	// Upsert article
	article := &domain.Article{
		URL:        req.ArticleURL,
		URLHash:    domain.URLHash(req.ArticleURL),
		Domain:     domain.ExtractDomain(req.ArticleURL),
		SourceName: req.SourceName,
	}
	if upsertErr := s.repo.UpsertArticle(ctx, article); upsertErr != nil {
		return fmt.Errorf("upsert article: %w", upsertErr)
	}

	// Insert event
	event := &domain.PipelineEvent{
		ArticleURL:            req.ArticleURL,
		Stage:                 req.Stage,
		OccurredAt:            req.OccurredAt,
		ServiceName:           req.ServiceName,
		Metadata:              req.Metadata,
		MetadataSchemaVersion: 1,
		IdempotencyKey:        req.IdempotencyKey,
	}
	if insertErr := s.repo.InsertEvent(ctx, event); insertErr != nil {
		return fmt.Errorf("insert event: %w", insertErr)
	}

	return nil
}

// IngestBatch validates and stores a batch of pipeline events.
func (s *PipelineService) IngestBatch(ctx context.Context, req *domain.BatchIngestRequest) (int, error) {
	ingested := 0
	for i := range req.Events {
		if ingestErr := s.Ingest(ctx, &req.Events[i]); ingestErr != nil {
			return ingested, fmt.Errorf("event %d: %w", i, ingestErr)
		}
		ingested++
	}
	return ingested, nil
}

// GetFunnel returns the pipeline funnel for a time range.
func (s *PipelineService) GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error) {
	stages, queryErr := s.repo.GetFunnel(ctx, from, to)
	if queryErr != nil {
		return nil, fmt.Errorf("get funnel: %w", queryErr)
	}

	return &domain.FunnelResponse{
		From:        from,
		To:          to,
		Stages:      stages,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (s *PipelineService) validateIngestRequest(req *domain.IngestRequest) error {
	if !req.Stage.IsValid() {
		return errors.New("invalid pipeline stage")
	}

	// Enforce UTC
	if req.OccurredAt.Location() != time.UTC {
		return errors.New("occurred_at must be in UTC")
	}

	now := time.Now().UTC()

	// Reject future timestamps
	if req.OccurredAt.After(now) {
		return errors.New("occurred_at must not be in the future")
	}

	// Reject timestamps older than 24h
	if now.Sub(req.OccurredAt) > maxEventAge {
		return errors.New("occurred_at must not be more than 24 hours in the past")
	}

	return nil
}

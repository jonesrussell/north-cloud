package workers

import (
	"context"
	"time"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
)

// Scheduler polls the database for content whose scheduled_at time has arrived
// and enqueues it for real-time publishing.
type Scheduler struct {
	repo      *database.Repository
	queue     *orchestrator.PriorityQueue
	log       logger.Logger
	interval  time.Duration
	batchSize int
}

// NewScheduler creates a new scheduler.
func NewScheduler(
	repo *database.Repository,
	queue *orchestrator.PriorityQueue,
	log logger.Logger,
	interval time.Duration,
	batchSize int,
) *Scheduler {
	return &Scheduler{
		repo:      repo,
		queue:     queue,
		log:       log,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Run starts the scheduling loop until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("Scheduler started", logger.Duration("interval", s.interval))

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Scheduler shutting down")
			return
		case <-ticker.C:
			s.processDueContent(ctx)
		}
	}
}

func (s *Scheduler) processDueContent(ctx context.Context) {
	content, err := s.repo.GetDueScheduledContent(ctx, s.batchSize)
	if err != nil {
		s.log.Error("Failed to fetch due scheduled content", logger.Error(err))
		return
	}

	if len(content) == 0 {
		return
	}

	s.log.Info("Processing scheduled content", logger.Int("count", len(content)))

	for i := range content {
		s.processScheduledItem(ctx, &content[i])
	}
}

func (s *Scheduler) processScheduledItem(ctx context.Context, msg *domain.PublishMessage) {
	if err := s.repo.MarkContentPublished(ctx, msg.ContentID); err != nil {
		s.log.Error("Failed to mark content as published",
			logger.String("content_id", msg.ContentID),
			logger.Error(err),
		)
		return
	}

	if len(msg.Targets) == 0 {
		s.log.Warn("Scheduled content has no targets, skipping",
			logger.String("content_id", msg.ContentID),
		)
		return
	}

	for _, target := range msg.Targets {
		job := orchestrator.PublishJob{
			ContentID: msg.ContentID,
			Platform:  target.Platform,
			Account:   target.Account,
			Message:   msg,
		}
		if !s.queue.EnqueueRealtime(job) {
			s.log.Error("Realtime queue full, dropping scheduled job",
				logger.String("content_id", msg.ContentID),
				logger.String("platform", target.Platform),
			)
		}
	}

	s.log.Info("Scheduled content triggered",
		logger.String("content_id", msg.ContentID),
		logger.String("type", string(msg.Type)),
		logger.Int("targets", len(msg.Targets)),
	)
}

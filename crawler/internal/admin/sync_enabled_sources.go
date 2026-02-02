package admin

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultStaggerMinutes      = 5
	defaultMaxRetries          = 3
	defaultRetryBackoffSeconds = 60

	// Sync action results
	actionCreated      = "created"
	actionResumed      = "resumed"
	actionAlreadyHas   = "already_has_job"
	actionSkippedParse = "skipped_parse"
	actionError        = "error"
)

// SyncReport is the JSON response for the sync endpoint.
type SyncReport struct {
	// Counts for quick overview
	SourcesSeen    int `json:"sources_seen"`
	SourcesEnabled int `json:"sources_enabled"`

	// Action results
	Created       []string `json:"created"`
	AlreadyHasJob []string `json:"already_has_job"`
	Resumed       []string `json:"resumed"`

	// Skipped sources
	SkippedDisabled              []string `json:"skipped_disabled"`
	SkippedRateLimitParseFailure []string `json:"skipped_rate_limit_parse_failure"` // Deprecated: now uses default

	// Errors encountered
	Errors []string `json:"errors,omitempty"`
}

// SyncEnabledSourcesHandler handles POST /api/v1/admin/sync-enabled-sources.
type SyncEnabledSourcesHandler struct {
	SourcesClient    sources.Client
	JobRepo          *database.JobRepository
	ScheduleComputer *job.ScheduleComputer
	Logger           infralogger.Logger
	Stagger          time.Duration
}

// NewSyncEnabledSourcesHandler creates a new sync handler.
func NewSyncEnabledSourcesHandler(
	sourcesClient sources.Client,
	jobRepo *database.JobRepository,
	scheduleComputer *job.ScheduleComputer,
	logger infralogger.Logger,
	stagger time.Duration,
) *SyncEnabledSourcesHandler {
	if stagger <= 0 {
		stagger = defaultStaggerMinutes * time.Minute
	}
	return &SyncEnabledSourcesHandler{
		SourcesClient:    sourcesClient,
		JobRepo:          jobRepo,
		ScheduleComputer: scheduleComputer,
		Logger:           logger,
		Stagger:          stagger,
	}
}

// SyncEnabledSources is the Gin handler for the sync endpoint.
func (h *SyncEnabledSourcesHandler) SyncEnabledSources(c *gin.Context) {
	ctx := c.Request.Context()

	sourceList, err := h.SourcesClient.ListSources(ctx)
	if err != nil {
		h.Logger.Error("Failed to list sources", infralogger.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list sources"})
		return
	}

	report := SyncReport{
		SourcesSeen:                  len(sourceList),
		Created:                      []string{},
		AlreadyHasJob:                []string{},
		Resumed:                      []string{},
		SkippedDisabled:              []string{},
		SkippedRateLimitParseFailure: []string{}, // Deprecated but kept for compatibility
		Errors:                       []string{},
	}

	h.Logger.Info("Starting source sync",
		infralogger.Int("sources_seen", len(sourceList)),
	)

	enabled := make([]*sources.SourceListItem, 0, len(sourceList))
	for _, s := range sourceList {
		if s.Enabled {
			enabled = append(enabled, s)
		} else {
			report.SkippedDisabled = append(report.SkippedDisabled, s.ID.String())
		}
	}

	report.SourcesEnabled = len(enabled)
	n := len(enabled)
	now := time.Now()

	h.Logger.Info("Processing enabled sources",
		infralogger.Int("enabled_count", n),
		infralogger.Int("disabled_count", len(report.SkippedDisabled)),
	)

	for _, src := range enabled {
		action := h.processEnabledSource(ctx, src, n, now)
		switch action {
		case actionCreated:
			report.Created = append(report.Created, src.ID.String())
		case actionResumed:
			report.Resumed = append(report.Resumed, src.ID.String())
		case actionAlreadyHas:
			report.AlreadyHasJob = append(report.AlreadyHasJob, src.ID.String())
		case actionError:
			report.Errors = append(report.Errors, src.ID.String())
		case actionSkippedParse:
			report.SkippedRateLimitParseFailure = append(report.SkippedRateLimitParseFailure, src.ID.String())
		}
	}

	h.Logger.Info("Source sync completed",
		infralogger.Int("sources_seen", report.SourcesSeen),
		infralogger.Int("sources_enabled", report.SourcesEnabled),
		infralogger.Int("created", len(report.Created)),
		infralogger.Int("already_has_job", len(report.AlreadyHasJob)),
		infralogger.Int("resumed", len(report.Resumed)),
		infralogger.Int("skipped_disabled", len(report.SkippedDisabled)),
		infralogger.Int("errors", len(report.Errors)),
	)

	c.JSON(http.StatusOK, report)
}

// processEnabledSource handles one enabled source; returns action for report.
func (h *SyncEnabledSourcesHandler) processEnabledSource(
	ctx context.Context,
	src *sources.SourceListItem,
	n int,
	now time.Time,
) string {
	rateLimit := parseRateLimitInt(src.RateLimit)
	if src.RateLimit == "" {
		h.Logger.Debug("Source has no rate_limit, using default",
			infralogger.String("source_id", src.ID.String()),
			infralogger.Int("default_rate_limit", defaultRateLimit),
		)
	}

	existingJob, findErr := h.JobRepo.FindBySourceID(ctx, src.ID)
	if findErr != nil && !errors.Is(findErr, database.ErrJobNotFoundBySourceID) {
		h.Logger.Error("Job lookup failed",
			infralogger.String("source_id", src.ID.String()),
			infralogger.Error(findErr),
		)
		return actionError
	}

	if existingJob != nil {
		if existingJob.IsPaused {
			existingJob.IsPaused = false
			existingJob.Status = "scheduled"
			nextRun := now
			existingJob.NextRunAt = &nextRun
			if updateErr := h.JobRepo.Update(ctx, existingJob); updateErr != nil {
				h.Logger.Error("Failed to resume job",
					infralogger.String("source_id", src.ID.String()),
					infralogger.Error(updateErr),
				)
				return actionError
			}
			return actionResumed
		}
		return actionAlreadyHas
	}

	offset := stableHash(src.ID.String()) % n
	if offset < 0 {
		offset = -offset
	}
	nextRun := now.Add(time.Duration(offset) * h.Stagger)

	schedule := h.ScheduleComputer.ComputeSchedule(job.ScheduleInput{
		RateLimit: rateLimit,
		MaxDepth:  src.MaxDepth,
		Priority:  src.Priority,
	})

	sourceName := src.Name
	newJob := &domain.Job{
		ID:                  uuid.New().String(),
		SourceID:            src.ID.String(),
		SourceName:          &sourceName,
		URL:                 src.URL,
		IntervalMinutes:     &schedule.IntervalMinutes,
		IntervalType:        schedule.IntervalType,
		NextRunAt:           &nextRun,
		Status:              "scheduled",
		AutoManaged:         true,
		Priority:            schedule.NumericPriority,
		ScheduleEnabled:     true,
		MaxRetries:          defaultMaxRetries,
		RetryBackoffSeconds: defaultRetryBackoffSeconds,
		SchedulerVersion:    1,
	}

	if upsertErr := h.JobRepo.UpsertAutoManaged(ctx, newJob); upsertErr != nil {
		h.Logger.Error("Failed to create job",
			infralogger.String("source_id", src.ID.String()),
			infralogger.Error(upsertErr),
		)
		return actionError
	}
	return actionCreated
}

// stableHash returns a deterministic hash of s for stagger offset.
func stableHash(s string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum32())
}

// defaultRateLimit is used when a source has no rate_limit configured.
const defaultRateLimit = 10

// parseRateLimitInt extracts an integer from source-manager rate_limit string (e.g. "1s", "10/s").
// Returns defaultRateLimit if the string is empty or unparseable.
func parseRateLimitInt(rateLimit string) int {
	if rateLimit == "" {
		return defaultRateLimit
	}
	var rate int
	_, err := fmt.Sscanf(rateLimit, "%d", &rate)
	if err != nil || rate <= 0 {
		return defaultRateLimit
	}
	return rate
}

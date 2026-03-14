package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	defaultBackfillLimit = 0 // 0 = no limit (all sources)
	backfillJobStatus    = "scheduled"
)

// BackfillIndigenousReport is the JSON response for the backfill endpoint.
type BackfillIndigenousReport struct {
	SourcesFound   int                     `json:"sources_found"`
	JobsDispatched int                     `json:"jobs_dispatched"`
	Region         string                  `json:"region,omitempty"`
	DryRun         bool                    `json:"dry_run"`
	Sources        []BackfillSourceSummary `json:"sources"`
	Errors         []string                `json:"errors,omitempty"`
}

// BackfillSourceSummary describes a source included in the backfill.
type BackfillSourceSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Region     string `json:"region"`
	RenderMode string `json:"render_mode"`
}

// BackfillIndigenousHandler handles POST /api/v1/backfill/indigenous.
type BackfillIndigenousHandler struct {
	SourcesClient    sources.Client
	JobRepo          *database.JobRepository
	ScheduleComputer *job.ScheduleComputer
	Logger           infralogger.Logger
	Stagger          time.Duration
}

// NewBackfillIndigenousHandler creates a new backfill handler.
func NewBackfillIndigenousHandler(
	sourcesClient sources.Client,
	jobRepo *database.JobRepository,
	scheduleComputer *job.ScheduleComputer,
	logger infralogger.Logger,
	stagger time.Duration,
) *BackfillIndigenousHandler {
	if stagger <= 0 {
		stagger = defaultStaggerMinutes * time.Minute
	}
	return &BackfillIndigenousHandler{
		SourcesClient:    sourcesClient,
		JobRepo:          jobRepo,
		ScheduleComputer: scheduleComputer,
		Logger:           logger,
		Stagger:          stagger,
	}
}

// BackfillIndigenous triggers recrawl jobs for indigenous sources.
func (h *BackfillIndigenousHandler) BackfillIndigenous(c *gin.Context) {
	ctx := c.Request.Context()
	region := c.Query("region")
	dryRun := c.Query("dry_run") == "true"
	limit := ParseBackfillLimit(c.Query("limit"))

	allIndigenous, err := h.SourcesClient.ListIndigenousSources(ctx)
	if err != nil {
		h.Logger.Error("Failed to list indigenous sources for backfill", infralogger.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list sources"})
		return
	}

	indigenous := FilterIndigenousSources(allIndigenous, region, limit)

	report := h.buildReport(ctx, indigenous, region, dryRun)

	h.Logger.Info("Indigenous backfill completed",
		infralogger.Int("sources_found", report.SourcesFound),
		infralogger.Int("jobs_dispatched", report.JobsDispatched),
		infralogger.String("region", region),
		infralogger.Bool("dry_run", dryRun),
	)

	c.JSON(http.StatusOK, report)
}

// buildReport creates the backfill report, optionally dispatching jobs.
func (h *BackfillIndigenousHandler) buildReport(
	ctx context.Context,
	indigenous []*sources.SourceListItem,
	region string,
	dryRun bool,
) BackfillIndigenousReport {
	report := BackfillIndigenousReport{
		SourcesFound: len(indigenous),
		Region:       region,
		DryRun:       dryRun,
		Sources:      make([]BackfillSourceSummary, 0, len(indigenous)),
		Errors:       []string{},
	}

	now := time.Now()
	n := len(indigenous)

	for _, src := range indigenous {
		regionVal := ""
		if src.IndigenousRegion != nil {
			regionVal = *src.IndigenousRegion
		}
		report.Sources = append(report.Sources, BackfillSourceSummary{
			ID:         src.ID.String(),
			Name:       src.Name,
			Region:     regionVal,
			RenderMode: src.RenderMode,
		})

		if dryRun {
			continue
		}

		if dispatched := h.dispatchJob(ctx, src, n, now); dispatched {
			report.JobsDispatched++
		} else {
			report.Errors = append(report.Errors, src.ID.String())
		}
	}

	return report
}

// dispatchJob creates a one-time crawl job for a source.
func (h *BackfillIndigenousHandler) dispatchJob(
	ctx context.Context,
	src *sources.SourceListItem,
	n int,
	now time.Time,
) bool {
	return DispatchBackfillJob(ctx, src, n, now, h.Stagger, h.ScheduleComputer, h.JobRepo, h.Logger)
}

// FilterIndigenousSources filters sources by enabled state, optionally by region, with limit.
// It also skips sources with no indigenous_region for defensive correctness when the caller
// passes a mixed list (e.g. in tests); the dedicated ListIndigenousSources endpoint guarantees
// all results have indigenous_region set.
func FilterIndigenousSources(
	allSources []*sources.SourceListItem,
	region string,
	limit int,
) []*sources.SourceListItem {
	result := make([]*sources.SourceListItem, 0)
	for _, src := range allSources {
		if !src.Enabled || src.IndigenousRegion == nil {
			continue
		}
		if region != "" && *src.IndigenousRegion != region {
			continue
		}
		result = append(result, src)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ParseBackfillLimit parses the limit query parameter.
func ParseBackfillLimit(s string) int {
	if s == "" {
		return defaultBackfillLimit
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultBackfillLimit
	}
	return n
}

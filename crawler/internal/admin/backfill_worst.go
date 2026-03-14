package admin

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/naming"
)

// Default parameters for worst-sources backfill.
const (
	defaultWorstLimit        = 20
	defaultMinWordCount      = 100
	defaultDryRunWorstSuffix = "true"
)

// ESSearcher defines the Elasticsearch operations needed by the backfill handler.
type ESSearcher interface {
	SearchDocuments(ctx context.Context, index string, query map[string]any, result any) error
	IndexExists(ctx context.Context, index string) (bool, error)
}

// WorstSourceReport is the JSON response for the worst-sources backfill endpoint.
type WorstSourceReport struct {
	SourcesFound   int                  `json:"sources_found"`
	JobsDispatched int                  `json:"jobs_dispatched"`
	DryRun         bool                 `json:"dry_run"`
	MinWordCount   int                  `json:"min_word_count"`
	Sources        []WorstSourceSummary `json:"sources"`
	Errors         []string             `json:"errors,omitempty"`
}

// WorstSourceSummary describes a source in the worst-sources report.
type WorstSourceSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	AvgWordCount float64 `json:"avg_word_count"`
	DocCount     int64   `json:"doc_count"`
}

// BackfillWorstSourcesHandler handles POST /api/v1/backfill/worst-sources.
type BackfillWorstSourcesHandler struct {
	SourcesClient    sources.Client
	ESSearcher       ESSearcher
	JobRepo          *database.JobRepository
	ScheduleComputer *job.ScheduleComputer
	Logger           infralogger.Logger
	Stagger          time.Duration
}

// NewBackfillWorstSourcesHandler creates a new worst-sources backfill handler.
func NewBackfillWorstSourcesHandler(
	sourcesClient sources.Client,
	esSearcher ESSearcher,
	jobRepo *database.JobRepository,
	scheduleComputer *job.ScheduleComputer,
	logger infralogger.Logger,
	stagger time.Duration,
) *BackfillWorstSourcesHandler {
	if stagger <= 0 {
		stagger = defaultStaggerMinutes * time.Minute
	}
	return &BackfillWorstSourcesHandler{
		SourcesClient:    sourcesClient,
		ESSearcher:       esSearcher,
		JobRepo:          jobRepo,
		ScheduleComputer: scheduleComputer,
		Logger:           logger,
		Stagger:          stagger,
	}
}

// BackfillWorstSources handles the POST request for worst-sources backfill.
func (h *BackfillWorstSourcesHandler) BackfillWorstSources(c *gin.Context) {
	ctx := c.Request.Context()
	dryRun := c.DefaultQuery("dry_run", defaultDryRunWorstSuffix) == "true"
	limit := ParseIntParam(c.Query("limit"), defaultWorstLimit)
	minWordCount := ParseIntParam(c.Query("min_word_count"), defaultMinWordCount)

	allSources, err := h.SourcesClient.ListSources(ctx)
	if err != nil {
		h.Logger.Error("Failed to list sources for worst-sources backfill", infralogger.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list sources"})
		return
	}

	enabledSources := FilterEnabled(allSources)
	ranked := h.rankByAvgWordCount(ctx, enabledSources)

	if limit > 0 && limit < len(ranked) {
		ranked = ranked[:limit]
	}

	report := h.buildWorstReport(ctx, ranked, dryRun, minWordCount)

	h.Logger.Info("Worst-sources backfill completed",
		infralogger.Int("sources_found", report.SourcesFound),
		infralogger.Int("jobs_dispatched", report.JobsDispatched),
		infralogger.Bool("dry_run", dryRun),
	)

	c.JSON(http.StatusOK, report)
}

// rankedSource holds a source with its ES stats for sorting.
type rankedSource struct {
	Source       *sources.SourceListItem
	AvgWordCount float64
	DocCount     int64
}

// rankByAvgWordCount queries ES for each source's avg word_count and sorts ascending.
func (h *BackfillWorstSourcesHandler) rankByAvgWordCount(
	ctx context.Context,
	srcs []*sources.SourceListItem,
) []rankedSource {
	ranked := make([]rankedSource, 0, len(srcs))

	for _, src := range srcs {
		avgWC, docCount, err := h.querySourceStats(ctx, src.Name)
		if err != nil {
			h.Logger.Warn("Failed to query ES stats for source",
				infralogger.String("source_name", src.Name),
				infralogger.Error(err),
			)
			// Include with zero stats so it ranks as worst
			ranked = append(ranked, rankedSource{Source: src, AvgWordCount: 0, DocCount: 0})
			continue
		}
		ranked = append(ranked, rankedSource{Source: src, AvgWordCount: avgWC, DocCount: docCount})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].AvgWordCount < ranked[j].AvgWordCount
	})

	return ranked
}

// esAggResponse represents the ES aggregation response structure.
type esAggResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
	Aggregations struct {
		AvgWordCount struct {
			Value *float64 `json:"value"`
		} `json:"avg_word_count"`
	} `json:"aggregations"`
}

// querySourceStats queries ES for a source's average word_count and doc count.
func (h *BackfillWorstSourcesHandler) querySourceStats(
	ctx context.Context,
	sourceName string,
) (avgWC float64, docCount int64, err error) {
	indexName := naming.RawContentIndex(sourceName)

	exists, err := h.ESSearcher.IndexExists(ctx, indexName)
	if err != nil {
		return 0, 0, fmt.Errorf("check index %s: %w", indexName, err)
	}
	if !exists {
		return 0, 0, nil
	}

	query := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"avg_word_count": map[string]any{
				"avg": map[string]any{
					"field": "word_count",
				},
			},
		},
	}

	var resp esAggResponse
	if searchErr := h.ESSearcher.SearchDocuments(ctx, indexName, query, &resp); searchErr != nil {
		return 0, 0, fmt.Errorf("search %s: %w", indexName, searchErr)
	}

	if resp.Aggregations.AvgWordCount.Value != nil {
		avgWC = *resp.Aggregations.AvgWordCount.Value
	}

	return avgWC, resp.Hits.Total.Value, nil
}

// buildWorstReport creates the report, optionally dispatching crawl jobs.
func (h *BackfillWorstSourcesHandler) buildWorstReport(
	ctx context.Context,
	ranked []rankedSource,
	dryRun bool,
	minWordCount int,
) WorstSourceReport {
	report := WorstSourceReport{
		SourcesFound: len(ranked),
		DryRun:       dryRun,
		MinWordCount: minWordCount,
		Sources:      make([]WorstSourceSummary, 0, len(ranked)),
		Errors:       []string{},
	}

	now := time.Now()
	n := len(ranked)

	for _, r := range ranked {
		report.Sources = append(report.Sources, WorstSourceSummary{
			ID:           r.Source.ID.String(),
			Name:         r.Source.Name,
			AvgWordCount: r.AvgWordCount,
			DocCount:     r.DocCount,
		})

		if dryRun {
			continue
		}

		if dispatched := h.dispatchWorstJob(ctx, r.Source, n, now); dispatched {
			report.JobsDispatched++
		} else {
			report.Errors = append(report.Errors, r.Source.ID.String())
		}
	}

	return report
}

// dispatchWorstJob creates a one-time crawl job for a worst-performing source.
func (h *BackfillWorstSourcesHandler) dispatchWorstJob(
	ctx context.Context,
	src *sources.SourceListItem,
	n int,
	now time.Time,
) bool {
	return DispatchBackfillJob(ctx, src, n, now, h.Stagger, h.ScheduleComputer, h.JobRepo, h.Logger)
}

// ValidationReport is the JSON response for the validation report endpoint.
type ValidationReport struct {
	Sources         []ValidationSourceSummary `json:"sources"`
	OverallPassRate float64                   `json:"overall_pass_rate"`
}

// ValidationSourceSummary describes a source's validation status.
type ValidationSourceSummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	TotalDocs       int64   `json:"total_docs"`
	ValidDocs       int64   `json:"valid_docs"`
	Percentage      float64 `json:"percentage"`
	PassesThreshold bool    `json:"passes_threshold"`
}

// esCountResponse represents the ES count response structure.
type esCountResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
}

// ValidationReport handles GET /api/v1/backfill/validation-report.
func (h *BackfillWorstSourcesHandler) GetValidationReport(c *gin.Context) {
	ctx := c.Request.Context()
	minWordCount := ParseIntParam(c.Query("min_word_count"), defaultMinWordCount)

	allSources, err := h.SourcesClient.ListSources(ctx)
	if err != nil {
		h.Logger.Error("Failed to list sources for validation report", infralogger.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list sources"})
		return
	}

	enabledSources := FilterEnabled(allSources)
	report := h.buildValidationReport(ctx, enabledSources, minWordCount)

	c.JSON(http.StatusOK, report)
}

// buildValidationReport builds the validation report for all enabled sources.
func (h *BackfillWorstSourcesHandler) buildValidationReport(
	ctx context.Context,
	srcs []*sources.SourceListItem,
	minWordCount int,
) ValidationReport {
	summaries := make([]ValidationSourceSummary, 0, len(srcs))
	passCount := 0

	for _, src := range srcs {
		summary := h.queryValidationStats(ctx, src, minWordCount)
		summaries = append(summaries, summary)
		if summary.PassesThreshold {
			passCount++
		}
	}

	overallPassRate := 0.0
	if len(summaries) > 0 {
		overallPassRate = float64(passCount) / float64(len(summaries))
	}

	return ValidationReport{
		Sources:         summaries,
		OverallPassRate: overallPassRate,
	}
}

// validationThreshold is the minimum percentage of valid docs for a source to pass.
const validationThreshold = 0.5

// queryValidationStats queries ES for a source's total and valid doc counts.
func (h *BackfillWorstSourcesHandler) queryValidationStats(
	ctx context.Context,
	src *sources.SourceListItem,
	minWordCount int,
) ValidationSourceSummary {
	indexName := naming.RawContentIndex(src.Name)
	summary := ValidationSourceSummary{
		ID:   src.ID.String(),
		Name: src.Name,
	}

	exists, err := h.ESSearcher.IndexExists(ctx, indexName)
	if err != nil || !exists {
		return summary
	}

	// Query total docs
	totalQuery := map[string]any{
		"size":             0,
		"track_total_hits": true,
	}
	var totalResp esCountResponse
	if searchErr := h.ESSearcher.SearchDocuments(ctx, indexName, totalQuery, &totalResp); searchErr != nil {
		h.Logger.Warn("Failed to query total docs",
			infralogger.String("source_name", src.Name),
			infralogger.Error(searchErr),
		)
		return summary
	}
	summary.TotalDocs = totalResp.Hits.Total.Value

	// Query docs with word_count > minWordCount
	validQuery := map[string]any{
		"size":             0,
		"track_total_hits": true,
		"query": map[string]any{
			"range": map[string]any{
				"word_count": map[string]any{
					"gt": minWordCount,
				},
			},
		},
	}
	var validResp esCountResponse
	if searchErr := h.ESSearcher.SearchDocuments(ctx, indexName, validQuery, &validResp); searchErr != nil {
		h.Logger.Warn("Failed to query valid docs",
			infralogger.String("source_name", src.Name),
			infralogger.Error(searchErr),
		)
		return summary
	}
	summary.ValidDocs = validResp.Hits.Total.Value

	if summary.TotalDocs > 0 {
		summary.Percentage = float64(summary.ValidDocs) / float64(summary.TotalDocs)
	}
	summary.PassesThreshold = summary.Percentage >= validationThreshold

	return summary
}

// FilterEnabled returns only enabled sources.
func FilterEnabled(allSources []*sources.SourceListItem) []*sources.SourceListItem {
	result := make([]*sources.SourceListItem, 0, len(allSources))
	for _, src := range allSources {
		if src.Enabled {
			result = append(result, src)
		}
	}
	return result
}

// ParseIntParam parses an integer query param with a default.
func ParseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

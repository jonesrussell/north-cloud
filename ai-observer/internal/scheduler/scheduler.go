// Package scheduler provides the polling loop and cost-ceiling budget for the AI observer.
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	// tokensPerEvent is the estimated token cost per event for budget pre-check.
	tokensPerEvent = 50
	// categoryTimeout is the per-category deadline to prevent indefinite goroutine blocking.
	categoryTimeout = 5 * time.Minute
)

// Config holds scheduler configuration.
type Config struct {
	IntervalSeconds      int
	MaxTokensPerInterval int
	WindowDuration       time.Duration
	DryRun               bool
	DriftIntervalSeconds int
	DriftWindowDuration  time.Duration
}

// Budget is a thread-safe token budget for a single polling interval.
// Note: deductions are based on pre-estimated cost (len(events)*tokensPerEvent),
// not actual API token usage. Actual spend is recorded in each Insight.TokensUsed
// but is not fed back into this budget. The ceiling is therefore a conservative
// pre-check, not a precise accounting.
type Budget struct {
	mu        sync.Mutex
	max       int
	remaining int
}

// NewBudget creates a Budget with the given max tokens.
func NewBudget(maxTokens int) *Budget {
	return &Budget{max: maxTokens, remaining: maxTokens}
}

// Deduct attempts to deduct n tokens from the budget.
// Returns true if the deduction succeeded, false if budget is insufficient.
func (b *Budget) Deduct(n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining < n {
		return false
	}
	b.remaining -= n
	return true
}

// Reset restores the budget to its max for the next interval.
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.remaining = b.max
}

// categoryResult holds the output of a single category pass.
type categoryResult struct {
	insights []category.Insight
	err      error
}

// Scheduler runs category passes on a ticker and emits insights.
type Scheduler struct {
	fastCategories []category.Category
	slowCategories []category.Category
	writer         *insights.Writer
	provider       provider.LLMProvider
	cfg            Config
	log            logger.Logger
}

// New creates a new Scheduler with fast (frequent) and slow (drift) category slices.
func New(
	fastCategories []category.Category,
	slowCategories []category.Category,
	writer *insights.Writer,
	p provider.LLMProvider,
	cfg Config,
) *Scheduler {
	return &Scheduler{
		fastCategories: fastCategories,
		slowCategories: slowCategories,
		writer:         writer,
		provider:       p,
		cfg:            cfg,
	}
}

// WithLogger sets a logger on the scheduler (optional).
func (s *Scheduler) WithLogger(log logger.Logger) *Scheduler {
	s.log = log
	return s
}

// Run starts the polling loop and blocks until ctx is cancelled.
// Uses dual tickers: fast for classifier categories, slow for drift categories.
func (s *Scheduler) Run(ctx context.Context) {
	fastInterval := time.Duration(s.cfg.IntervalSeconds) * time.Second
	fastTicker := time.NewTicker(fastInterval)
	defer fastTicker.Stop()

	s.logInfo("Scheduler started",
		logger.Int("fast_interval_seconds", s.cfg.IntervalSeconds),
		logger.Int("slow_interval_seconds", s.cfg.DriftIntervalSeconds),
	)
	s.RunOnce(ctx)

	if s.cfg.DriftIntervalSeconds <= 0 || len(s.slowCategories) == 0 {
		s.runFastOnly(ctx, fastTicker)
		return
	}

	slowInterval := time.Duration(s.cfg.DriftIntervalSeconds) * time.Second
	slowTicker := time.NewTicker(slowInterval)
	defer slowTicker.Stop()

	s.RunDrift(ctx)
	s.runDualTicker(ctx, fastTicker, slowTicker)
}

func (s *Scheduler) runFastOnly(ctx context.Context, fastTicker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			s.logInfo("Scheduler stopping")
			return
		case <-fastTicker.C:
			s.RunOnce(ctx)
		}
	}
}

func (s *Scheduler) runDualTicker(
	ctx context.Context,
	fastTicker *time.Ticker,
	slowTicker *time.Ticker,
) {
	for {
		select {
		case <-ctx.Done():
			s.logInfo("Scheduler stopping")
			return
		case <-fastTicker.C:
			s.RunOnce(ctx)
		case <-slowTicker.C:
			s.RunDrift(ctx)
		}
	}
}

// RunOnce executes one polling cycle for fast categories.
func (s *Scheduler) RunOnce(ctx context.Context) {
	s.runCategories(ctx, s.fastCategories, s.cfg.WindowDuration)
}

// RunDrift executes one polling cycle for slow (drift) categories.
func (s *Scheduler) RunDrift(ctx context.Context) {
	s.runCategories(ctx, s.slowCategories, s.cfg.DriftWindowDuration)
}

func (s *Scheduler) runCategories(
	ctx context.Context,
	cats []category.Category,
	window time.Duration,
) {
	if len(cats) == 0 {
		return
	}

	budget := NewBudget(s.cfg.MaxTokensPerInterval)
	results := make(chan categoryResult, len(cats))

	var wg sync.WaitGroup
	for _, cat := range cats {
		wg.Add(1)
		go func(c category.Category) {
			defer wg.Done()
			ins, runErr := s.runCategory(ctx, c, budget, window)
			results <- categoryResult{insights: ins, err: runErr}
		}(cat)
	}

	wg.Wait()
	close(results)

	allInsights := s.collectInsights(results)
	s.writeInsights(ctx, allInsights)
}

func (s *Scheduler) collectInsights(results <-chan categoryResult) []category.Insight {
	allInsights := make([]category.Insight, 0)
	for r := range results {
		if r.err != nil {
			s.logError("category error", r.err)
			continue
		}
		allInsights = append(allInsights, r.insights...)
	}
	return allInsights
}

func (s *Scheduler) writeInsights(ctx context.Context, allInsights []category.Insight) {
	if s.cfg.DryRun {
		s.logInfo("Dry run: would write insights", logger.Int("count", len(allInsights)))
		return
	}
	if s.writer == nil || len(allInsights) == 0 {
		return
	}
	if err := s.writer.WriteAll(ctx, allInsights); err != nil {
		s.logError("write insights error", err)
	} else {
		s.logInfo("Insights written", logger.Int("count", len(allInsights)))
	}
}

func (s *Scheduler) runCategory(
	ctx context.Context,
	cat category.Category,
	budget *Budget,
	window time.Duration,
) ([]category.Insight, error) {
	if s.cfg.DryRun {
		s.logInfo("Dry run: skipping LLM call", logger.String("category", cat.Name()))
		return nil, nil
	}

	catCtx, cancel := context.WithTimeout(ctx, categoryTimeout)
	defer cancel()

	events, err := cat.Sample(catCtx, window)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	estimatedTokens := len(events) * tokensPerEvent
	if !budget.Deduct(estimatedTokens) {
		s.logInfo("budget_exceeded",
			logger.String("category", cat.Name()),
			logger.Int("estimated_tokens", estimatedTokens),
		)
		return nil, nil
	}

	return cat.Analyze(catCtx, events, s.provider)
}

func (s *Scheduler) logInfo(msg string, fields ...logger.Field) {
	if s.log != nil {
		s.log.Info(msg, fields...)
	}
}

func (s *Scheduler) logError(msg string, err error, fields ...logger.Field) {
	if s.log != nil {
		allFields := append([]logger.Field{logger.Error(err)}, fields...)
		s.log.Error(msg, allFields...)
	}
}

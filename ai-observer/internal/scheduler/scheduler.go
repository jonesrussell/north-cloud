// Package scheduler provides the polling loop and cost-ceiling budget for the AI observer.
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	"github.com/north-cloud/infrastructure/logger"
)

const (
	// tokensPerEvent is the estimated token cost per event for budget pre-check.
	tokensPerEvent = 50
)

// Config holds scheduler configuration.
type Config struct {
	IntervalSeconds      int
	MaxTokensPerInterval int
	WindowDuration       time.Duration
	DryRun               bool
}

// Budget is a thread-safe token budget for a single polling interval.
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
	categories []category.Category
	writer     *insights.Writer
	provider   provider.LLMProvider
	cfg        Config
	log        logger.Logger
}

// New creates a new Scheduler.
func New(
	categories []category.Category,
	writer *insights.Writer,
	p provider.LLMProvider,
	cfg Config,
) *Scheduler {
	return &Scheduler{
		categories: categories,
		writer:     writer,
		provider:   p,
		cfg:        cfg,
	}
}

// WithLogger sets a logger on the scheduler (optional).
func (s *Scheduler) WithLogger(log logger.Logger) *Scheduler {
	s.log = log
	return s
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	interval := time.Duration(s.cfg.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logInfo("Scheduler started", logger.Int("interval_seconds", s.cfg.IntervalSeconds))
	s.RunOnce(ctx) // run immediately on start

	for {
		select {
		case <-ctx.Done():
			s.logInfo("Scheduler stopping")
			return
		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

// RunOnce executes one polling cycle: sample all categories in parallel, collect insights, write.
func (s *Scheduler) RunOnce(ctx context.Context) {
	if len(s.categories) == 0 {
		return
	}

	budget := NewBudget(s.cfg.MaxTokensPerInterval)
	results := make(chan categoryResult, len(s.categories))

	var wg sync.WaitGroup
	for _, cat := range s.categories {
		wg.Add(1)
		go func(c category.Category) {
			defer wg.Done()
			ins, err := s.runCategory(ctx, c, budget)
			results <- categoryResult{insights: ins, err: err}
		}(cat)
	}

	wg.Wait()
	close(results)

	allInsights := s.collectInsights(results)
	s.writeInsights(ctx, allInsights)
}

func (s *Scheduler) collectInsights(results <-chan categoryResult) []category.Insight {
	allInsights := make([]category.Insight, 0, len(s.categories))
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

func (s *Scheduler) runCategory(ctx context.Context, cat category.Category, budget *Budget) ([]category.Insight, error) {
	if s.cfg.DryRun {
		s.logInfo("Dry run: skipping LLM call", logger.String("category", cat.Name()))
		return nil, nil
	}

	events, err := cat.Sample(ctx, s.cfg.WindowDuration)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	estimatedTokens := len(events) * tokensPerEvent
	if !budget.Deduct(estimatedTokens) {
		s.logInfo("budget exhausted, skipping category", logger.String("category", cat.Name()))
		return nil, nil
	}

	return cat.Analyze(ctx, events, s.provider)
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

package processor

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting for operations
type RateLimiter struct {
	limiter *rate.Limiter
	logger  Logger
}

// NewRateLimiter creates a new rate limiter
// rps: requests per second
// burst: maximum burst size
func NewRateLimiter(rps, burst int, logger Logger) *RateLimiter {
	if rps <= 0 {
		rps = 100 // Default 100 requests per second
	}
	if burst <= 0 {
		burst = rps // Default burst equals rps
	}

	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
		logger:  logger,
	}
}

// Wait waits until rate limit allows the operation
func (r *RateLimiter) Wait(ctx context.Context) error {
	if err := r.limiter.Wait(ctx); err != nil {
		r.logger.Warn("Rate limiter wait failed", "error", err)
		return err
	}
	return nil
}

// Allow checks if an operation is allowed without waiting
func (r *RateLimiter) Allow() bool {
	return r.limiter.Allow()
}

// Reserve reserves a token and returns a reservation
func (r *RateLimiter) Reserve() *rate.Reservation {
	return r.limiter.Reserve()
}

// SetLimit updates the rate limit
func (r *RateLimiter) SetLimit(rps int) {
	r.limiter.SetLimit(rate.Limit(rps))
	r.logger.Info("Rate limit updated", "new_rps", rps)
}

// SetBurst updates the burst size
func (r *RateLimiter) SetBurst(burst int) {
	r.limiter.SetBurst(burst)
	r.logger.Info("Burst size updated", "new_burst", burst)
}

// RateLimitedProcessor wraps a batch processor with rate limiting
type RateLimitedProcessor struct {
	processor *BatchProcessor
	esLimiter *RateLimiter
	dbLimiter *RateLimiter
	logger    Logger
}

// NewRateLimitedProcessor creates a processor with rate limiting
func NewRateLimitedProcessor(
	processor *BatchProcessor,
	esRPS int,
	dbRPS int,
	logger Logger,
) *RateLimitedProcessor {
	return &RateLimitedProcessor{
		processor: processor,
		esLimiter: NewRateLimiter(esRPS, esRPS, logger),
		dbLimiter: NewRateLimiter(dbRPS, dbRPS, logger),
		logger:    logger,
	}
}

// ProcessWithRateLimit processes items with rate limiting
func (r *RateLimitedProcessor) ProcessWithRateLimit(
	ctx context.Context,
	items []*domain.RawContent,
) ([]*ProcessResult, error) {
	// Wait for ES rate limit (we'll be querying ES)
	if err := r.esLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Process the batch
	results, err := r.processor.Process(ctx, items)
	if err != nil {
		return nil, err
	}

	// Wait for DB rate limit (we'll be writing to DB)
	if err := r.dbLimiter.Wait(ctx); err != nil {
		r.logger.Warn("DB rate limit wait failed, continuing anyway", "error", err)
		// Don't fail the operation, just log the warning
	}

	return results, nil
}

// GetESLimiter returns the Elasticsearch rate limiter
func (r *RateLimitedProcessor) GetESLimiter() *RateLimiter {
	return r.esLimiter
}

// GetDBLimiter returns the database rate limiter
func (r *RateLimitedProcessor) GetDBLimiter() *RateLimiter {
	return r.dbLimiter
}

// RateLimitedPoller wraps a poller with rate limiting
type RateLimitedPoller struct {
	poller      *Poller
	rateLimiter *RateLimiter
	logger      Logger
}

// NewRateLimitedPoller creates a poller with rate limiting
func NewRateLimitedPoller(
	poller *Poller,
	pollRPS int,
	logger Logger,
) *RateLimitedPoller {
	return &RateLimitedPoller{
		poller:      poller,
		rateLimiter: NewRateLimiter(pollRPS, pollRPS, logger),
		logger:      logger,
	}
}

// Start starts the rate-limited poller
func (r *RateLimitedPoller) Start(ctx context.Context) error {
	return r.poller.Start(ctx)
}

// Stop stops the poller
func (r *RateLimitedPoller) Stop() {
	r.poller.Stop()
}

// IsRunning returns whether the poller is running
func (r *RateLimitedPoller) IsRunning() bool {
	return r.poller.IsRunning()
}

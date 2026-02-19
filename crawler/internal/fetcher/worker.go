package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
)

// Status codes used in processURL for HTTP response handling.
const (
	statusOK           = 200
	statusNotModified  = 304
	statusNotFound     = 404
	statusTooManyReqs  = 429
	statusServerErrLow = 500
)

// Reason strings for dead URL classification.
const (
	reasonRobotsBlocked    = "robots_blocked"
	reasonNotFound         = "not_found"
	reasonTooManyRedirects = "too_many_redirects"
)

// maxResponseBodyBytes limits the size of fetched page responses.
const maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MB

// ErrNoURLAvailable is returned when no URL is available in the frontier.
// This mirrors database.ErrNoURLAvailable to avoid importing the database package.
var ErrNoURLAvailable = errors.New("no URL available in frontier")

// FetchedParams contains the parameters for marking a URL as fetched.
type FetchedParams struct {
	ContentHash  *string
	ETag         *string
	LastModified *string
}

// FrontierClaimer claims and updates URLs in the frontier.
type FrontierClaimer interface {
	Claim(ctx context.Context) (*domain.FrontierURL, error)
	UpdateFetched(ctx context.Context, id string, params FetchedParams) error
	UpdateFetchedWithFinalURL(ctx context.Context, id, finalURL string, params FetchedParams) error
	UpdateFailed(ctx context.Context, id, lastError string, maxRetries int) error
	UpdateDead(ctx context.Context, id, reason string) error
}

// HostUpdater records fetch activity per host.
type HostUpdater interface {
	UpdateLastFetch(ctx context.Context, host string) error
}

// RobotsAllower checks robots.txt compliance.
type RobotsAllower interface {
	IsAllowed(ctx context.Context, rawURL string) (bool, error)
}

// ContentIndexer indexes extracted content to Elasticsearch.
type ContentIndexer interface {
	Index(ctx context.Context, content *ExtractedContent) error
}

// WorkerLogger provides structured logging.
type WorkerLogger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
}

// WorkerPoolConfig configures the worker pool.
type WorkerPoolConfig struct {
	WorkerCount     int
	UserAgent       string
	MaxRetries      int
	ClaimRetryDelay time.Duration
	RequestTimeout  time.Duration
	// HTTPClient is the client used for fetches. If nil, a default client with RequestTimeout is used.
	HTTPClient *http.Client
}

// WorkerPool manages a pool of fetch workers that process URLs from the frontier.
type WorkerPool struct {
	frontier        FrontierClaimer
	hostUpdater     HostUpdater
	robots          RobotsAllower
	extractor       *ContentExtractor
	indexer         ContentIndexer
	log             WorkerLogger
	httpClient      *http.Client
	userAgent       string
	workerCount     int
	maxRetries      int
	claimRetryDelay time.Duration
}

// NewWorkerPool creates a new worker pool with the given dependencies and configuration.
func NewWorkerPool(
	claimer FrontierClaimer,
	hostUpdater HostUpdater,
	robots RobotsAllower,
	extractor *ContentExtractor,
	indexer ContentIndexer,
	log WorkerLogger,
	cfg WorkerPoolConfig,
) *WorkerPool {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.RequestTimeout}
	}
	return &WorkerPool{
		frontier:        claimer,
		hostUpdater:     hostUpdater,
		robots:          robots,
		extractor:       extractor,
		indexer:         indexer,
		log:             log,
		httpClient:      client,
		userAgent:       cfg.UserAgent,
		workerCount:     cfg.WorkerCount,
		maxRetries:      cfg.MaxRetries,
		claimRetryDelay: cfg.ClaimRetryDelay,
	}
}

// Start launches workerCount goroutines. Blocks until ctx is cancelled.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.log.Info("starting worker pool", "worker_count", wp.workerCount)

	var wg sync.WaitGroup

	for i := range wp.workerCount {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()
			wp.worker(ctx, workerID)
		}(i)
	}

	wg.Wait()
	wp.log.Info("worker pool stopped")

	return nil
}

// worker is a single worker goroutine loop.
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	wp.log.Info("worker started", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			wp.log.Info("worker stopping", "worker_id", workerID)
			return
		default:
		}

		if shouldReturn := wp.claimAndProcess(ctx, workerID); shouldReturn {
			return
		}
	}
}

// claimAndProcess attempts to claim a URL and process it.
// Returns true if the worker should exit (context cancelled).
func (wp *WorkerPool) claimAndProcess(ctx context.Context, workerID int) bool {
	claimed, err := wp.frontier.Claim(ctx)
	if errors.Is(err, ErrNoURLAvailable) {
		return wp.sleepOrCancel(ctx)
	}

	if err != nil {
		wp.log.Error("claim failed", "worker_id", workerID, "error", err.Error())
		return wp.sleepOrCancel(ctx)
	}

	if processErr := wp.ProcessURL(ctx, claimed); processErr != nil {
		wp.log.Error("process failed",
			"worker_id", workerID,
			"url", claimed.URL,
			"error", processErr.Error(),
		)
	}

	return false
}

// sleepOrCancel sleeps for the claim retry delay or returns true if the context is cancelled.
func (wp *WorkerPool) sleepOrCancel(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(wp.claimRetryDelay):
		return false
	}
}

// ProcessURL handles a single claimed URL through the full pipeline.
func (wp *WorkerPool) ProcessURL(ctx context.Context, furl *domain.FrontierURL) error {
	allowed, robotsErr := wp.robots.IsAllowed(ctx, furl.URL)
	if robotsErr != nil {
		return fmt.Errorf("robots check: %w", robotsErr)
	}

	if !allowed {
		if updateErr := wp.frontier.UpdateDead(ctx, furl.ID, reasonRobotsBlocked); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL marked dead", "url", furl.URL, "reason", reasonRobotsBlocked)
		return nil
	}

	body, statusCode, finalURL, fetchErr := wp.fetchPage(ctx, furl)

	// Always update host last fetch time after any fetch attempt.
	wp.updateHostFetch(ctx, furl.Host)

	if fetchErr != nil {
		return wp.handleFetchError(ctx, furl, fetchErr)
	}

	return wp.handleStatusCode(ctx, furl, body, statusCode, finalURL)
}

// updateHostFetch records a fetch attempt for politeness tracking.
func (wp *WorkerPool) updateHostFetch(ctx context.Context, host string) {
	if hostErr := wp.hostUpdater.UpdateLastFetch(ctx, host); hostErr != nil {
		wp.log.Error("update host last fetch failed",
			"host", host,
			"error", hostErr.Error(),
		)
	}
}

// handleFetchError records a failed fetch in the frontier.
func (wp *WorkerPool) handleFetchError(ctx context.Context, furl *domain.FrontierURL, fetchErr error) error {
	lastError := fetchErr.Error()
	if errors.Is(fetchErr, ErrTooManyRedirects) {
		lastError = reasonTooManyRedirects
	}
	updateErr := wp.frontier.UpdateFailed(ctx, furl.ID, lastError, wp.maxRetries)
	if updateErr != nil {
		return fmt.Errorf("update failed after fetch error: %w", updateErr)
	}
	wp.log.Info("URL fetch failed", "url", furl.URL, "error", lastError)
	return nil
}

// handleStatusCode routes the HTTP response to the appropriate handler.
func (wp *WorkerPool) handleStatusCode(
	ctx context.Context,
	furl *domain.FrontierURL,
	body []byte,
	statusCode int,
	finalURL string,
) error {
	switch {
	case statusCode == statusOK:
		return wp.handleSuccess(ctx, furl, body, finalURL)
	case statusCode == statusNotModified:
		return wp.handleNotModified(ctx, furl, finalURL)
	case statusCode == statusNotFound:
		if updateErr := wp.frontier.UpdateDead(ctx, furl.ID, reasonNotFound); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL marked dead", "url", furl.URL, "reason", reasonNotFound)
		return nil
	case statusCode == statusTooManyReqs || statusCode >= statusServerErrLow:
		msg := fmt.Sprintf("http status %d", statusCode)
		if updateErr := wp.frontier.UpdateFailed(ctx, furl.ID, msg, wp.maxRetries); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL fetch failed", "url", furl.URL, "error", msg)
		return nil
	default:
		msg := fmt.Sprintf("unexpected http status %d", statusCode)
		if updateErr := wp.frontier.UpdateFailed(ctx, furl.ID, msg, wp.maxRetries); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL fetch failed", "url", furl.URL, "error", msg)
		return nil
	}
}

// handleSuccess extracts content, indexes it, and marks the URL as fetched.
func (wp *WorkerPool) handleSuccess(
	ctx context.Context,
	furl *domain.FrontierURL,
	body []byte,
	finalURL string,
) error {
	content, extractErr := wp.extractor.Extract(furl.SourceID, furl.URL, body)
	if extractErr != nil {
		return fmt.Errorf("extract content: %w", extractErr)
	}

	if indexErr := wp.indexer.Index(ctx, content); indexErr != nil {
		return fmt.Errorf("index content: %w", indexErr)
	}

	params := FetchedParams{
		ContentHash: &content.ContentHash,
	}

	if updateErr := wp.updateFetchedWithOptionalFinalURL(ctx, furl, finalURL, params); updateErr != nil {
		return updateErr
	}
	wp.log.Info("URL fetched successfully", "url", furl.URL)
	return nil
}

// handleNotModified marks the URL as fetched without indexing new content.
func (wp *WorkerPool) handleNotModified(ctx context.Context, furl *domain.FrontierURL, finalURL string) error {
	if updateErr := wp.updateFetchedWithOptionalFinalURL(ctx, furl, finalURL, FetchedParams{}); updateErr != nil {
		return updateErr
	}
	wp.log.Info("URL fetched successfully", "url", furl.URL)
	return nil
}

// updateFetchedWithOptionalFinalURL marks the URL as fetched. If finalURL normalizes to the same
// as the claimed URL, it calls UpdateFetched; otherwise UpdateFetchedWithFinalURL so the frontier
// stores the canonical URL. On normalization error, falls back to UpdateFetched.
func (wp *WorkerPool) updateFetchedWithOptionalFinalURL(
	ctx context.Context,
	furl *domain.FrontierURL,
	finalURL string,
	params FetchedParams,
) error {
	normFinal, normFinalErr := frontier.NormalizeURL(finalURL)
	normClaimed, normClaimedErr := frontier.NormalizeURL(furl.URL)
	if normFinalErr != nil || normClaimedErr != nil {
		return wp.frontier.UpdateFetched(ctx, furl.ID, params)
	}
	if normFinal == normClaimed {
		return wp.frontier.UpdateFetched(ctx, furl.ID, params)
	}
	return wp.frontier.UpdateFetchedWithFinalURL(ctx, furl.ID, finalURL, params)
}

// fetchPage performs the HTTP GET request for the given frontier URL.
// finalURL is the response request URL after redirects (resp.Request.URL).
func (wp *WorkerPool) fetchPage(
	ctx context.Context,
	furl *domain.FrontierURL,
) (body []byte, statusCode int, finalURL string, err error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, furl.URL, http.NoBody)
	if reqErr != nil {
		return nil, 0, "", fmt.Errorf("create request: %w", reqErr)
	}

	req.Header.Set("User-Agent", wp.userAgent)
	setConditionalHeaders(req, furl)

	resp, doErr := wp.httpClient.Do(req)
	if doErr != nil {
		return nil, 0, "", fmt.Errorf("http fetch: %w", doErr)
	}
	defer resp.Body.Close()

	finalURL = resp.Request.URL.String()
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes)

	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, resp.StatusCode, finalURL, fmt.Errorf("read response body: %w", readErr)
	}

	return body, resp.StatusCode, finalURL, nil
}

// setConditionalHeaders adds If-None-Match and If-Modified-Since headers
// when the frontier URL has cached ETag or LastModified values.
func setConditionalHeaders(req *http.Request, furl *domain.FrontierURL) {
	if furl.ETag != nil {
		req.Header.Set("If-None-Match", *furl.ETag)
	}

	if furl.LastModified != nil {
		req.Header.Set("If-Modified-Since", *furl.LastModified)
	}
}

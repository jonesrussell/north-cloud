package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	reasonRobotsBlocked          = "robots_blocked"
	reasonNotFound               = "not_found"
	reasonTooManyRedirects       = "too_many_redirects"
	reasonUnsupportedContentType = "unsupported_content_type"
	reasonBinaryURL              = "binary_url"
	reasonExtractFailed          = "extract_failed"
)

// renderModeDynamic is the render mode value indicating Playwright rendering is required.
const renderModeDynamic = "dynamic"

// renderedPageContentType is the Content-Type returned for Playwright-rendered pages.
const renderedPageContentType = "text/html; charset=utf-8"

// RenderedPage holds the result of a dynamic Playwright render.
type RenderedPage struct {
	HTML       string
	FinalURL   string
	StatusCode int
}

// PageRenderer renders a URL using a headless browser and returns the HTML.
type PageRenderer interface {
	Render(ctx context.Context, url string) (*RenderedPage, error)
}

// SourceRenderModeResolver looks up the render mode ("static" or "dynamic") for a source.
type SourceRenderModeResolver interface {
	GetRenderMode(ctx context.Context, sourceID string) (string, error)
}

// binaryExtensions are file extensions that indicate binary/non-content resources.
// These URLs are marked dead in the frontier to avoid repeated parse failures.
var binaryExtensions = []string{
	".pdf", ".xml", ".json", ".css", ".js",
	".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp",
	".woff", ".woff2", ".ttf", ".eot",
	".zip", ".gz", ".tar", ".rar",
	".mp3", ".mp4", ".wav", ".ogg", ".avi", ".mov",
	".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
}

// binaryPathSubstrings are URL path substrings that indicate binary download endpoints.
var binaryPathSubstrings = []string{
	"downloadmp3", "download.php", "downloadfile",
}

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
	// Renderer renders pages via a headless browser. Nil disables dynamic rendering.
	Renderer PageRenderer
	// ModeResolver resolves the render mode for a source. Nil disables dynamic rendering.
	ModeResolver SourceRenderModeResolver
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
	renderer        PageRenderer
	modeResolver    SourceRenderModeResolver
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
		renderer:        cfg.Renderer,
		modeResolver:    cfg.ModeResolver,
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

	body, statusCode, finalURL, contentType, fetchErr := wp.fetchWithRenderMode(ctx, furl)

	// Always update host last fetch time after any fetch attempt.
	wp.updateHostFetch(ctx, furl.Host)

	if fetchErr != nil {
		return wp.handleFetchError(ctx, furl, fetchErr)
	}

	return wp.handleStatusCode(ctx, furl, body, statusCode, finalURL, contentType)
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
	contentType string,
) error {
	switch {
	case statusCode == statusOK:
		return wp.handleSuccess(ctx, furl, body, finalURL, contentType)
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
// Non-HTML responses (PDFs, images, etc.) are marked dead to avoid repeated parse failures.
func (wp *WorkerPool) handleSuccess(
	ctx context.Context,
	furl *domain.FrontierURL,
	body []byte,
	finalURL string,
	contentType string,
) error {
	if !isHTMLContent(contentType) {
		if updateErr := wp.frontier.UpdateDead(ctx, furl.ID, reasonUnsupportedContentType); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL marked dead",
			"url", furl.URL,
			"reason", reasonUnsupportedContentType,
			"content_type", contentType,
		)
		return nil
	}

	if isBinaryURL(furl.URL) {
		if updateErr := wp.frontier.UpdateDead(ctx, furl.ID, reasonBinaryURL); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL marked dead",
			"url", furl.URL,
			"reason", reasonBinaryURL,
		)
		return nil
	}

	content, extractErr := wp.extractor.Extract(furl.SourceID, furl.URL, body)
	if extractErr != nil {
		// Extraction failures are permanent — the page structure won't change on retry.
		if updateErr := wp.frontier.UpdateDead(ctx, furl.ID, reasonExtractFailed); updateErr != nil {
			return updateErr
		}
		wp.log.Info("URL marked dead",
			"url", furl.URL,
			"reason", reasonExtractFailed,
			"error", extractErr.Error(),
		)
		return nil
	}

	if indexErr := wp.indexer.Index(ctx, content); indexErr != nil {
		// Indexing failures are transient (ES may be down) — use retry with backoff.
		if updateErr := wp.frontier.UpdateFailed(ctx, furl.ID, indexErr.Error(), wp.maxRetries); updateErr != nil {
			return fmt.Errorf("update failed after index error: %w", updateErr)
		}
		wp.log.Info("URL index failed", "url", furl.URL, "error", indexErr.Error())
		return nil
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

// fetchWithRenderMode fetches a page using the render worker for dynamic sources,
// or plain HTTP for static sources (or when rendering is not configured).
func (wp *WorkerPool) fetchWithRenderMode(
	ctx context.Context,
	furl *domain.FrontierURL,
) (body []byte, statusCode int, finalURL, contentType string, err error) {
	if wp.renderer != nil && wp.modeResolver != nil {
		mode, resolveErr := wp.modeResolver.GetRenderMode(ctx, furl.SourceID)
		if resolveErr != nil {
			return nil, 0, "", "", fmt.Errorf("resolve render mode: %w", resolveErr)
		}

		if mode == renderModeDynamic {
			page, renderErr := wp.renderer.Render(ctx, furl.URL)
			if renderErr != nil {
				return nil, 0, "", "", fmt.Errorf("render: %w", renderErr)
			}

			return []byte(page.HTML), page.StatusCode, page.FinalURL, renderedPageContentType, nil
		}
	}

	return wp.fetchPage(ctx, furl)
}

// fetchPage performs the HTTP GET request for the given frontier URL.
// finalURL is the response request URL after redirects (resp.Request.URL).
// contentType is the Content-Type header from the response.
func (wp *WorkerPool) fetchPage(
	ctx context.Context,
	furl *domain.FrontierURL,
) (body []byte, statusCode int, finalURL, contentType string, err error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, furl.URL, http.NoBody)
	if reqErr != nil {
		return nil, 0, "", "", fmt.Errorf("create request: %w", reqErr)
	}

	req.Header.Set("User-Agent", wp.userAgent)
	setConditionalHeaders(req, furl)

	resp, doErr := wp.httpClient.Do(req)
	if doErr != nil {
		return nil, 0, "", "", fmt.Errorf("http fetch: %w", doErr)
	}
	defer resp.Body.Close()

	finalURL = resp.Request.URL.String()
	contentType = resp.Header.Get("Content-Type")
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes)

	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, resp.StatusCode, finalURL, contentType, fmt.Errorf("read response body: %w", readErr)
	}

	return body, resp.StatusCode, finalURL, contentType, nil
}

// isHTMLContent returns true if the Content-Type header indicates an HTML response.
// An empty Content-Type is treated as HTML to handle servers that omit the header.
func isHTMLContent(contentType string) bool {
	if contentType == "" {
		return true
	}
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/html") || strings.Contains(ct, "xhtml")
}

// isBinaryURL returns true if the URL path has a binary file extension
// (e.g. .mp3, .pdf) or matches a known binary download pattern (e.g. downloadmp3.php).
func isBinaryURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	lowerPath := strings.ToLower(parsed.Path)
	for _, ext := range binaryExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	for _, substr := range binaryPathSubstrings {
		if strings.Contains(lowerPath, substr) {
			return true
		}
	}
	return false
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

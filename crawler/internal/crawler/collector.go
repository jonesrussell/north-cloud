package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/proxy"
	"github.com/gocolly/redisstorage"
	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

// Collector defaults
const (
	defaultRateLimit   = 2 * time.Second
	defaultParallelism = 2
	// RandomDelayDivisor is used to calculate random delay from rate limit
	RandomDelayDivisor = 2
)

// HTTP transport defaults
const (
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 10
	defaultIdleConnTimeout       = 90 * time.Second
	defaultResponseHeaderTimeout = 30 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
)

// Max depth defaults
const (
	// defaultMaxDepth is used when source MaxDepth is 0 (unset)
	defaultMaxDepth = 3
	// warnMaxDepth logs a warning when source MaxDepth exceeds this value
	warnMaxDepth = 5
)

// Progress logging configuration
const (
	// progressMilestoneInterval defines how often (in pages) to emit progress milestones.
	progressMilestoneInterval = 50
)

// Rule action values for URLFilters (source Rules).
const (
	ruleActionAllow    = "allow"
	ruleActionDisallow = "disallow"
)

// refererCtxKey is the request context key for the referer URL (set before Visit from link handler).
const refererCtxKey = "referer"

// retryCountKey is the request context key for HTTP retry count in OnError.
const retryCountKey = "retry_count"

// randomUserAgents is a small set of desktop browser user agents for UseRandomUserAgent.
var randomUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
}

// setupCollector configures the collector with the given source settings.
func (c *Crawler) setupCollector(ctx context.Context, source *configtypes.Source) error {
	maxDepth := c.resolveMaxDepth(source)
	opts := c.buildCollectorOptions(ctx, maxDepth, source)
	c.collector = colly.NewCollector(opts...)

	// Set Redis storage if enabled (graceful fallback to in-memory on failure)
	if storageErr := c.setupRedisStorage(); storageErr != nil {
		c.GetJobLogger().Warn(logs.CategoryLifecycle,
			"Failed to set Redis storage, using in-memory",
			logs.Err(storageErr),
		)
	}

	// Set up proxy rotation if enabled
	if proxyErr := c.setupProxyRotation(); proxyErr != nil {
		return fmt.Errorf("failed to set up proxy rotation: %w", proxyErr)
	}

	c.collector.SetRequestTimeout(c.cfg.RequestTimeout)

	// Parse and set rate limit (accepts "10s", "1m", or bare number as seconds e.g. "10")
	rateLimit := c.resolveRateLimit(source)
	if setErr := c.setRateLimit(rateLimit, rateLimit/RandomDelayDivisor); setErr != nil {
		return fmt.Errorf("failed to set rate limit: %w", setErr)
	}

	// Configure transport with TLS settings
	c.configureTransport()

	if c.cfg.TLS.InsecureSkipVerify {
		c.GetJobLogger().Warn(logs.CategoryLifecycle,
			"TLS certificate verification is disabled",
			logs.String("source", source.Name),
		)
	}

	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Collector configured",
		logs.Int("max_depth", maxDepth),
		logs.Duration("rate_limit", rateLimit),
		logs.Int("parallelism", defaultParallelism),
	)

	return nil
}

// resolveMaxDepth returns the effective max depth for the source, applying defaults and warnings.
func (c *Crawler) resolveMaxDepth(source *configtypes.Source) int {
	maxDepth := source.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
		c.GetJobLogger().Info(logs.CategoryLifecycle, "Using default max depth",
			logs.Int("max_depth", maxDepth),
		)
	}
	if maxDepth > warnMaxDepth {
		c.GetJobLogger().Warn(logs.CategoryLifecycle,
			"Max depth exceeds recommended limit",
			logs.Int("max_depth", maxDepth),
			logs.Int("recommended_max", warnMaxDepth),
		)
	}
	return maxDepth
}

// buildCollectorOptions builds the Colly collector options from source and config settings.
func (c *Crawler) buildCollectorOptions(
	ctx context.Context, maxDepth int, source *configtypes.Source,
) []colly.CollectorOption {
	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Setting up collector",
		logs.Int("max_depth", maxDepth),
		logs.Any("allowed_domains", source.AllowedDomains),
	)

	opts := []colly.CollectorOption{
		colly.StdlibContext(ctx),
		colly.MaxDepth(maxDepth),
		colly.Async(true),
		colly.ParseHTTPErrorResponse(),
		// Note: Not using AllowURLRevisit() to prevent excessive request queuing.
		// Each URL will only be crawled once, which significantly reduces Wait() time.
	}

	if !c.cfg.RespectRobotsTxt {
		opts = append(opts, colly.IgnoreRobotsTxt())
	}
	if !c.cfg.UseRandomUserAgent {
		opts = append(opts, colly.UserAgent(c.cfg.UserAgent))
	}
	if c.cfg.MaxBodySize > 0 {
		opts = append(opts, colly.MaxBodySize(c.cfg.MaxBodySize))
	}
	if c.cfg.DetectCharset {
		opts = append(opts, colly.DetectCharset())
	}
	if c.cfg.TraceHTTP {
		opts = append(opts, colly.TraceHTTP())
	}
	if c.cfg.MaxRequests > 0 {
		opts = append(opts, colly.MaxRequests(c.cfg.MaxRequests))
	}

	// URLFilters from source Rules: "allow" -> URLFilters, "disallow" -> DisallowedURLFilters
	allowRegexes, disallowRegexes := c.compileRuleFilters(source)
	if len(allowRegexes) > 0 {
		opts = append(opts, colly.URLFilters(allowRegexes...))
	}
	if len(disallowRegexes) > 0 {
		opts = append(opts, colly.DisallowedURLFilters(disallowRegexes...))
	}

	// Only set allowed domains if they are configured
	if len(source.AllowedDomains) > 0 {
		opts = append(opts, colly.AllowedDomains(source.AllowedDomains...))
	}

	return opts
}

// resolveRateLimit parses the source rate limit string or returns the default.
func (c *Crawler) resolveRateLimit(source *configtypes.Source) time.Duration {
	rateLimit, err := crawlerconfig.ParseRateLimit(source.RateLimit)
	if err != nil || source.RateLimit == "" {
		if source.RateLimit != "" {
			c.GetJobLogger().Error(logs.CategoryError,
				"Failed to parse rate limit, using default",
				logs.String("rate_limit", source.RateLimit),
				logs.Duration("default", defaultRateLimit),
				logs.Err(err),
			)
		}
		return defaultRateLimit
	}
	return rateLimit
}

// setupRedisStorage configures Redis-backed Colly storage if enabled and available.
func (c *Crawler) setupRedisStorage() error {
	if !c.cfg.RedisStorageEnabled || c.redisClient == nil {
		return nil
	}

	crawlCtx := c.getCrawlContext()
	prefix := "crawler:default"
	if crawlCtx != nil {
		prefix = "crawler:" + crawlCtx.SourceID
	}

	storage := &redisstorage.Storage{
		Address:  c.redisClient.Options().Addr,
		Password: c.redisClient.Options().Password,
		DB:       c.redisClient.Options().DB,
		Prefix:   prefix,
		Expires:  c.cfg.RedisStorageExpires,
	}

	if err := c.collector.SetStorage(storage); err != nil {
		return fmt.Errorf("failed to set Redis storage: %w", err)
	}

	c.GetJobLogger().Info(logs.CategoryLifecycle,
		"Redis storage enabled for Colly",
		logs.String("prefix", prefix),
		logs.Duration("expires", c.cfg.RedisStorageExpires),
	)
	return nil
}

// setupProxyRotation configures round-robin proxy rotation if enabled.
func (c *Crawler) setupProxyRotation() error {
	if !c.cfg.ProxiesEnabled || len(c.cfg.ProxyURLs) == 0 {
		return nil
	}

	rp, err := proxy.RoundRobinProxySwitcher(c.cfg.ProxyURLs...)
	if err != nil {
		return fmt.Errorf("failed to create proxy switcher: %w", err)
	}

	c.collector.SetProxyFunc(rp)

	c.GetJobLogger().Info(logs.CategoryLifecycle,
		"Proxy rotation enabled",
		logs.Int("proxy_count", len(c.cfg.ProxyURLs)),
	)
	return nil
}

// compileRuleFilters compiles source Rules into allow and disallow regex slices.
func (c *Crawler) compileRuleFilters(source *configtypes.Source) (allow, disallow []*regexp.Regexp) {
	for _, rule := range source.Rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			c.GetJobLogger().Warn(logs.CategoryLifecycle, "Skipping invalid rule pattern",
				logs.String("pattern", rule.Pattern),
				logs.String("action", rule.Action),
				logs.Err(err),
			)
			continue
		}
		switch strings.ToLower(strings.TrimSpace(rule.Action)) {
		case ruleActionAllow:
			allow = append(allow, re)
		case ruleActionDisallow:
			disallow = append(disallow, re)
		}
	}
	return allow, disallow
}

// responseHeadersCallback returns a callback that aborts non-HTML or oversized responses.
func (c *Crawler) responseHeadersCallback() func(*colly.Response) {
	return func(r *colly.Response) {
		contentType := strings.ToLower(strings.TrimSpace(r.Headers.Get("Content-Type")))
		isHTML := strings.HasPrefix(contentType, "text/html") ||
			strings.HasPrefix(contentType, "application/xhtml+xml") ||
			strings.Contains(contentType, "text/html")
		if contentType != "" && !isHTML {
			c.GetJobLogger().IncrementSkippedNonHTML()
			r.Request.Abort()
			return
		}
		if c.cfg.MaxBodySize > 0 {
			if cl := r.Headers.Get("Content-Length"); cl != "" {
				var contentLength int
				if _, err := fmt.Sscanf(cl, "%d", &contentLength); err == nil && contentLength > c.cfg.MaxBodySize {
					r.Request.Abort()
				}
			}
		}
	}
}

// responseCallback returns the OnResponse callback (archiving, trace logging, Cloudflare detection).
func (c *Crawler) responseCallback(ctx context.Context) func(*colly.Response) {
	return func(r *colly.Response) {
		jl := c.GetJobLogger()
		pageURL := r.Request.URL.String()

		// Record execution visibility metrics
		jl.RecordStatusCode(r.StatusCode)
		jl.IncrementRequestsTotal()
		jl.RecordBytes(int64(len(r.Body)))
		if r.Trace != nil {
			jl.RecordResponseTime(r.Trace.FirstByteDuration)
		}

		jl.Debug(logs.CategoryFetch, "Response received",
			logs.URL(pageURL),
			logs.Int("status", r.StatusCode),
		)
		if proxyURL := r.Request.ProxyURL; proxyURL != "" {
			jl.Debug(logs.CategoryFetch, "Request via proxy",
				logs.URL(pageURL),
				logs.String("proxy", proxyURL),
			)
		}
		if c.cfg.Debug && c.cfg.TraceHTTP && r.Trace != nil {
			jl.Debug(logs.CategoryFetch, "HTTP trace",
				logs.URL(pageURL),
				logs.Duration("connect", r.Trace.ConnectDuration),
				logs.Duration("first_byte", r.Trace.FirstByteDuration),
			)
		}
		if c.isCloudflareChallenge(r) {
			jl.IncrementCloudflare()
			jl.Warn(logs.CategoryFetch, "Cloudflare challenge detected",
				logs.URL(pageURL),
				logs.Int("status", r.StatusCode),
			)
		}
		if r.StatusCode == http.StatusTooManyRequests {
			jl.IncrementRateLimit()
		}
		// Capture hash for start URLs (adaptive scheduling)
		c.captureStartURLHash(pageURL, r.Body)

		if c.archiver != nil {
			task := &archive.UploadTask{
				HTML:       r.Body,
				URL:        pageURL,
				SourceName: c.state.CurrentSource(),
				StatusCode: r.StatusCode,
				Headers:    convertHeaders(r.Headers),
				Timestamp:  time.Now(),
				Ctx:        ctx,
			}
			if err := c.archiver.Archive(ctx, task); err != nil {
				jl.Warn(logs.CategoryError, "Failed to archive HTML",
					logs.URL(pageURL),
					logs.Err(err),
				)
			}
		}
	}
}

// requestCallback returns the OnRequest callback (abort checks, Referer, RandomUserAgent).
func (c *Crawler) requestCallback(ctx context.Context) func(*colly.Request) {
	return func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
			return
		case <-c.signals.AbortChannel():
			r.Abort()
			return
		default:
			if c.cfg.UseReferer {
				if referer := r.Ctx.Get(refererCtxKey); referer != "" {
					r.Headers.Set("Referer", referer)
				}
			}
			if c.cfg.UseRandomUserAgent {
				r.Headers.Set("User-Agent", randomUserAgents[rand.Intn(len(randomUserAgents))])
			}
			c.GetJobLogger().Debug(logs.CategoryFetch, "Visiting URL",
				logs.URL(r.URL.String()),
			)
		}
	}
}

// setupCallbacks configures the collector's callbacks.
func (c *Crawler) setupCallbacks(ctx context.Context) {
	c.collector.OnResponseHeaders(c.responseHeadersCallback())
	c.collector.OnResponse(c.responseCallback(ctx))
	c.collector.OnRequest(c.requestCallback(ctx))

	// Set up HTML processing
	c.collector.OnHTML("html", func(e *colly.HTMLElement) {
		c.ProcessHTML(e)
	})

	// Set up error handling
	c.collector.OnError(c.handleCrawlError)

	// Set up link following
	c.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// Check if we should stop processing before following links
		select {
		case <-ctx.Done():
			return
		case <-c.signals.AbortChannel():
			return
		default:
			c.linkHandler.HandleLink(e)
		}
	})

	// Set up scraped callback for logging/metrics
	// This callback allows us to track link counts after a page is fully processed
	c.collector.OnScraped(func(r *colly.Response) {
		// Note: OnScraped fires AFTER the request completes, so we can't abort here.
		// This callback is only for post-processing (logging, metrics, etc.)
		pageURL := r.Request.URL.String()
		c.GetJobLogger().Debug(logs.CategoryFetch, "Page processed",
			logs.URL(pageURL),
		)

		// Track pages crawled for heartbeat and milestone progress
		c.GetJobLogger().IncrementPagesCrawled()

		// Emit milestone progress logs every N pages
		summary := c.GetJobLogger().BuildSummary()
		pagesCrawled := summary.PagesCrawled
		if pagesCrawled > 0 && pagesCrawled%progressMilestoneInterval == 0 {
			c.GetJobLogger().Info(logs.CategoryMetrics,
				fmt.Sprintf("Milestone: %d pages crawled, %d items extracted",
					pagesCrawled, summary.ItemsExtracted),
				logs.Int64("pages_crawled", pagesCrawled),
				logs.Int64("items_extracted", summary.ItemsExtracted),
			)
		}
	})
}

// handleCrawlError handles crawl errors with appropriate logging levels and optional HTTP retry.
func (c *Crawler) handleCrawlError(r *colly.Response, visitErr error) {
	jl := c.GetJobLogger()
	errMsg := visitErr.Error()

	// Record status code for error responses
	if r != nil && r.StatusCode > 0 {
		jl.RecordStatusCode(r.StatusCode)
	}

	// Check if this is an expected/non-critical error (log at debug)
	if c.isExpectedCrawlError(visitErr, errMsg) {
		c.trackExpectedErrorMetrics(jl, errMsg)
		jl.Debug(logs.CategoryError, "Expected error while crawling",
			logs.URL(r.Request.URL.String()),
			logs.Int("status", r.StatusCode),
			logs.String("error", errMsg),
		)
		return
	}

	// Check if this is a timeout (log at warn level - common but still an issue)
	isTimeout := strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "Timeout") ||
		strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "context deadline exceeded")

	if isTimeout {
		jl.Warn(logs.CategoryError, "Timeout while crawling",
			logs.URL(r.Request.URL.String()),
			logs.Int("status", r.StatusCode),
		)
		jl.IncrementRequestsFailed()
		jl.RecordErrorCategory("timeout")
		c.IncrementError()
		jl.IncrementErrors()
		return
	}

	// Transient errors: retry up to HTTPRetryMax times
	if c.tryHTTPRetry(r, visitErr) {
		return
	}

	// Log actual errors (non-retryable or retries disabled)
	jl.Error(logs.CategoryError, "Crawl error",
		logs.URL(r.Request.URL.String()),
		logs.Int("status", r.StatusCode),
		logs.Err(visitErr),
	)

	jl.IncrementRequestsFailed()
	c.categorizeError(jl, r, visitErr)
	c.IncrementError()
	jl.IncrementErrors()
}

// isExpectedCrawlError returns true for expected/non-critical errors.
func (c *Crawler) isExpectedCrawlError(visitErr error, errMsg string) bool {
	return errors.Is(visitErr, ErrAlreadyVisited) ||
		errors.Is(visitErr, ErrMaxDepth) ||
		errors.Is(visitErr, ErrForbiddenDomain) ||
		strings.Contains(errMsg, "forbidden domain") ||
		strings.Contains(errMsg, "Forbidden domain") ||
		strings.Contains(errMsg, "max depth") ||
		strings.Contains(errMsg, "Max depth") ||
		strings.Contains(errMsg, "already visited") ||
		strings.Contains(errMsg, "Already visited") ||
		strings.Contains(errMsg, "Not following redirect")
}

// trackExpectedErrorMetrics records skip metrics for expected errors.
func (c *Crawler) trackExpectedErrorMetrics(jl logs.JobLogger, errMsg string) {
	lowerMsg := strings.ToLower(errMsg)
	if strings.Contains(lowerMsg, "max depth") {
		jl.IncrementSkippedMaxDepth()
	}
}

// categorizeError classifies an error into a category for metrics.
func (c *Crawler) categorizeError(jl logs.JobLogger, r *colly.Response, visitErr error) {
	errMsg := strings.ToLower(visitErr.Error())
	networkPatterns := []string{
		"connection refused", "connection reset", "no such host",
		"eof", "broken pipe", "i/o timeout", "connection timed out",
	}
	for _, p := range networkPatterns {
		if strings.Contains(errMsg, p) {
			jl.RecordErrorCategory("network")
			return
		}
	}
	if r != nil {
		switch {
		case r.StatusCode >= http.StatusInternalServerError:
			jl.RecordErrorCategory("http_server")
		case r.StatusCode >= http.StatusBadRequest:
			jl.RecordErrorCategory("http_client")
		}
	}
}

// tryHTTPRetry attempts to retry the request for transient errors.
// Returns true if it handled the error (retried or logged after exhausting retries), false if not retryable.
func (c *Crawler) tryHTTPRetry(r *colly.Response, visitErr error) bool {
	if !c.isTransientCrawlError(r, visitErr) || c.cfg.HTTPRetryMax <= 0 {
		return false
	}
	count := 0
	if v := r.Request.Ctx.GetAny(retryCountKey); v != nil {
		if n, ok := v.(int); ok {
			count = n
		}
	}
	if count >= c.cfg.HTTPRetryMax {
		jl := c.GetJobLogger()
		jl.Error(logs.CategoryError, "Crawl error after retries",
			logs.URL(r.Request.URL.String()),
			logs.Int("status", r.StatusCode),
			logs.Err(visitErr),
			logs.Int("retries", count),
		)
		jl.IncrementRequestsFailed()
		jl.RecordErrorCategory("http_server")
		c.IncrementError()
		jl.IncrementErrors()
		return true
	}
	r.Request.Ctx.Put(retryCountKey, count+1)
	time.Sleep(c.cfg.HTTPRetryDelay)
	if retryErr := r.Request.Retry(); retryErr != nil {
		c.GetJobLogger().Warn(logs.CategoryError, "Retry failed",
			logs.URL(r.Request.URL.String()),
			logs.Err(retryErr),
		)
		c.IncrementError()
		c.GetJobLogger().IncrementErrors()
	}
	return true
}

// isTransientCrawlError returns true if the error looks retryable (5xx, connection issues).
func (c *Crawler) isTransientCrawlError(r *colly.Response, visitErr error) bool {
	errMsg := strings.ToLower(visitErr.Error())
	transientPatterns := []string{
		"connection refused", "connection reset", "connection reset by peer",
		"temporary failure", "eof", "broken pipe", "no such host",
		"i/o timeout", "connection timed out",
	}
	for _, p := range transientPatterns {
		if strings.Contains(errMsg, p) {
			return true
		}
	}
	if r != nil && r.StatusCode >= 500 && r.StatusCode < 600 {
		return true
	}
	return false
}

// Collector Management Methods
// -----------------------------

// SetRateLimit sets the rate limit for the crawler.
func (c *Crawler) SetRateLimit(duration time.Duration) error {
	if c.collector == nil {
		return errors.New("collector is nil")
	}

	if duration < 0 {
		return errors.New("rate limit must be non-negative")
	}

	// Public API: set rate limit without random delay
	return c.setRateLimit(duration, 0)
}

// setRateLimit sets the rate limit on the collector.
func (c *Crawler) setRateLimit(delay, randomDelay time.Duration) error {
	err := c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       delay,
		RandomDelay: randomDelay,
		Parallelism: defaultParallelism,
	})
	if err != nil {
		return fmt.Errorf("failed to set rate limit: %w", err)
	}
	return nil
}

// configureTransport configures the HTTP transport with TLS settings from config.
func (c *Crawler) configureTransport() {
	c.collector.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.cfg.TLS.InsecureSkipVerify,
			MinVersion:         c.cfg.TLS.MinVersion,
			MaxVersion:         c.cfg.TLS.MaxVersion,
		},
		DisableKeepAlives:     false,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
	})
}

// SetCollector sets the collector for the crawler.
func (c *Crawler) SetCollector(collector *colly.Collector) {
	c.collector = collector
}

// isCloudflareChallenge detects if a response is a Cloudflare challenge page.
// Cloudflare challenge pages typically have:
// - Cf-Ray header (Cloudflare request ID)
// - Cf-Mitigated: challenge header
// - "Just a moment..." or similar challenge content in body
func (c *Crawler) isCloudflareChallenge(r *colly.Response) bool {
	// Check for Cloudflare headers
	hasCfRay := r.Headers.Get("Cf-Ray") != ""
	hasCfMitigated := strings.ToLower(r.Headers.Get("Cf-Mitigated")) == "challenge"

	// Check for challenge content in body (common Cloudflare challenge text)
	bodyText := strings.ToLower(string(r.Body))
	hasChallengeContent := strings.Contains(bodyText, "just a moment") ||
		strings.Contains(bodyText, "checking your browser") ||
		strings.Contains(bodyText, "ddos protection by cloudflare") ||
		strings.Contains(bodyText, "please wait...")

	// Also check for Cloudflare server header
	server := strings.ToLower(r.Headers.Get("Server"))
	hasCloudflareServer := strings.Contains(server, "cloudflare")

	// Cloudflare challenge is likely if we have Cf-Ray + Cf-Mitigated: challenge
	// OR if we have challenge content with Cloudflare headers
	return (hasCfRay && hasCfMitigated) || (hasChallengeContent && (hasCfRay || hasCloudflareServer))
}

// convertHeaders converts Colly response headers to a map[string]string.
func convertHeaders(headers *http.Header) map[string]string {
	result := make(map[string]string)
	if headers == nil {
		return result
	}
	for key, values := range *headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// captureStartURLHash stores the SHA-256 hash of a start URL's response body.
func (c *Crawler) captureStartURLHash(pageURL string, body []byte) {
	crawlCtx := c.getCrawlContext()
	if crawlCtx == nil || crawlCtx.Source == nil {
		return
	}

	if !c.isStartURL(pageURL, crawlCtx.Source) {
		return
	}

	hash := adaptive.ComputeHash(body)
	c.startURLHashesMu.Lock()
	c.startURLHashes[crawlCtx.SourceID] = hash
	c.startURLHashesMu.Unlock()
}

// isStartURL checks if the given URL is a start URL for the source.
// Uses trimmed trailing-slash comparison to handle redirect normalization.
func (c *Crawler) isStartURL(
	pageURL string, source *configtypes.Source,
) bool {
	normalized := strings.TrimRight(pageURL, "/")
	if normalized == strings.TrimRight(source.URL, "/") {
		return true
	}
	for _, u := range source.StartURLs {
		if normalized == strings.TrimRight(u, "/") {
			return true
		}
	}
	return false
}

package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
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

// setupCollector configures the collector with the given source settings.
func (c *Crawler) setupCollector(source *configtypes.Source) error {
	maxDepth := source.MaxDepth

	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Setting up collector",
		logs.Int("max_depth", maxDepth),
		logs.Any("allowed_domains", source.AllowedDomains),
	)

	opts := []colly.CollectorOption{
		colly.MaxDepth(maxDepth),
		colly.Async(true),
		colly.ParseHTTPErrorResponse(),
		colly.IgnoreRobotsTxt(),
		colly.UserAgent(c.cfg.UserAgent),
		// Note: Not using AllowURLRevisit() to prevent excessive request queuing.
		// Each URL will only be crawled once, which significantly reduces Wait() time.
	}

	// Only set allowed domains if they are configured
	if len(source.AllowedDomains) > 0 {
		opts = append(opts, colly.AllowedDomains(source.AllowedDomains...))
	}

	c.collector = colly.NewCollector(opts...)

	// Parse and set rate limit
	rateLimit, err := time.ParseDuration(source.RateLimit)
	if err != nil {
		c.GetJobLogger().Error(logs.CategoryError, "Failed to parse rate limit, using default",
			logs.String("rate_limit", source.RateLimit),
			logs.Duration("default", defaultRateLimit),
			logs.Err(err),
		)
		rateLimit = defaultRateLimit
	}

	if setErr := c.setRateLimit(rateLimit, rateLimit/RandomDelayDivisor); setErr != nil {
		return fmt.Errorf("failed to set rate limit: %w", setErr)
	}

	// Configure transport with TLS settings
	c.configureTransport()

	if c.cfg.TLS.InsecureSkipVerify {
		c.GetJobLogger().Warn(logs.CategoryLifecycle, "TLS certificate verification is disabled",
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

// setupCallbacks configures the collector's callbacks.
func (c *Crawler) setupCallbacks(ctx context.Context) {
	// Set up response callback
	c.collector.OnResponse(func(r *colly.Response) {
		pageURL := r.Request.URL.String()
		c.GetJobLogger().Debug(logs.CategoryFetch, "Response received",
			logs.URL(pageURL),
			logs.Int("status", r.StatusCode),
		)

		// Detect Cloudflare challenge pages
		if c.isCloudflareChallenge(r) {
			c.GetJobLogger().Warn(logs.CategoryFetch, "Cloudflare challenge detected",
				logs.URL(pageURL),
				logs.Int("status", r.StatusCode),
			)
		}

		// Archive HTML to MinIO if archiver is enabled
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
				c.GetJobLogger().Warn(logs.CategoryError, "Failed to archive HTML",
					logs.URL(pageURL),
					logs.Err(err),
				)
			}
		}
	})

	// Set up request callback
	c.collector.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
			return
		case <-c.signals.AbortChannel():
			r.Abort()
			return
		default:
			c.GetJobLogger().Debug(logs.CategoryFetch, "Visiting URL",
				logs.URL(r.URL.String()),
			)
		}
	})

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

		// Note: Link counts are tracked via the HandleLink callback logging.
		// The actual count of queued links would require accessing colly's internal queue,
		// which is not exposed. We rely on the "Discovered link" logs for diagnostics.
	})
}

// handleCrawlError handles crawl errors with appropriate logging levels.
func (c *Crawler) handleCrawlError(r *colly.Response, visitErr error) {
	errMsg := visitErr.Error()

	// Check if this is an expected/non-critical error (log at debug)
	isExpectedError := errors.Is(visitErr, ErrAlreadyVisited) ||
		errors.Is(visitErr, ErrMaxDepth) ||
		errors.Is(visitErr, ErrForbiddenDomain) ||
		strings.Contains(errMsg, "forbidden domain") ||
		strings.Contains(errMsg, "Forbidden domain") ||
		strings.Contains(errMsg, "max depth") ||
		strings.Contains(errMsg, "Max depth") ||
		strings.Contains(errMsg, "already visited") ||
		strings.Contains(errMsg, "Already visited") ||
		strings.Contains(errMsg, "Not following redirect")

	if isExpectedError {
		// These are expected conditions, log at debug level
		c.GetJobLogger().Debug(logs.CategoryError, "Expected error while crawling",
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
		// Timeouts are common when crawling, log at warn level
		c.GetJobLogger().Warn(logs.CategoryError, "Timeout while crawling",
			logs.URL(r.Request.URL.String()),
			logs.Int("status", r.StatusCode),
		)
		c.IncrementError()
		return
	}

	// Log actual errors
	c.GetJobLogger().Error(logs.CategoryError, "Crawl error",
		logs.URL(r.Request.URL.String()),
		logs.Int("status", r.StatusCode),
		logs.Err(visitErr),
	)

	c.IncrementError()
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

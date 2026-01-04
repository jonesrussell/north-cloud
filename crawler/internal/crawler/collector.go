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

	c.logger.Debug("Setting up collector",
		"max_depth", maxDepth,
		"allowed_domains", source.AllowedDomains)

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
		c.logger.Error("Failed to parse rate limit, using default",
			"rate_limit", source.RateLimit,
			"default", defaultRateLimit,
			"error", err)
		rateLimit = defaultRateLimit
	}

	if setErr := c.setRateLimit(rateLimit, rateLimit/RandomDelayDivisor); setErr != nil {
		return fmt.Errorf("failed to set rate limit: %w", setErr)
	}

	// Configure transport with TLS settings
	c.configureTransport()

	if c.cfg.TLS.InsecureSkipVerify {
		c.logger.Warn("TLS certificate verification is disabled. This is not recommended for production use.",
			"component", "crawler",
			"source", source.Name,
			"warning", "This makes HTTPS connections vulnerable to man-in-the-middle attacks")
	}

	c.logger.Debug("Collector configured",
		"max_depth", maxDepth,
		"allowed_domains", source.AllowedDomains,
		"rate_limit", rateLimit,
		"parallelism", defaultParallelism)

	return nil
}

// setupCallbacks configures the collector's callbacks.
func (c *Crawler) setupCallbacks(ctx context.Context) {
	// Set up response callback
	c.collector.OnResponse(func(r *colly.Response) {
		c.logger.Debug("Received response",
			"url", r.Request.URL.String(),
			"status", r.StatusCode,
			"headers", r.Headers)

		// Archive HTML to MinIO if archiver is enabled
		if c.archiver != nil {
			task := &archive.UploadTask{
				HTML:       r.Body,
				URL:        r.Request.URL.String(),
				SourceName: c.state.CurrentSource(),
				StatusCode: r.StatusCode,
				Headers:    convertHeaders(r.Headers),
				Timestamp:  time.Now(),
				Ctx:        ctx,
			}

			if err := c.archiver.Archive(ctx, task); err != nil {
				c.logger.Warn("Failed to archive HTML",
					"url", r.Request.URL.String(),
					"error", err)
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
			c.logger.Debug("Visiting URL",
				"url", r.URL.String())
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
	c.collector.OnScraped(func(r *colly.Response) {
		// Note: OnScraped fires AFTER the request completes, so we can't abort here.
		// This callback is only for post-processing (logging, metrics, etc.)
		c.logger.Debug("Finished processing page",
			"url", r.Request.URL.String())
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
		c.logger.Debug("Expected error while crawling",
			"url", r.Request.URL.String(),
			"status", r.StatusCode,
			"error", errMsg)
		return
	}

	// Check if this is a timeout (log at warn level - common but still an issue)
	isTimeout := strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "Timeout") ||
		strings.Contains(errMsg, "deadline exceeded") ||
		strings.Contains(errMsg, "context deadline exceeded")

	if isTimeout {
		// Timeouts are common when crawling, log at warn level
		c.logger.Warn("Timeout while crawling",
			"url", r.Request.URL.String(),
			"status", r.StatusCode,
			"error", errMsg)
		c.IncrementError()
		return
	}

	// Log actual errors
	c.logger.Error("Error while crawling",
		"url", r.Request.URL.String(),
		"status", r.StatusCode,
		"error", visitErr)

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
			InsecureSkipVerify: c.cfg.TLS.InsecureSkipVerify, //nolint:gosec // Configurable for development/testing
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

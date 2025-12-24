package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
)

// Collector defaults
const (
	defaultRateLimit             = 2 * time.Second
	defaultMaxDepth              = 3
	defaultMaxConcurrency        = 2
	defaultParallelism           = 2
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 10
	defaultIdleConnTimeout       = 90 * time.Second
	defaultResponseHeaderTimeout = 30 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
)

// CollectorConfig holds configuration for the collector.
type CollectorConfig struct {
	RateLimit      time.Duration
	MaxDepth       int
	MaxConcurrency int
}

// NewCollectorConfig creates a new collector configuration.
func NewCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		RateLimit:      defaultRateLimit,
		MaxDepth:       defaultMaxDepth,
		MaxConcurrency: defaultMaxConcurrency,
	}
}

// Validate validates the collector configuration.
func (c *CollectorConfig) Validate() error {
	if c.RateLimit < 0 {
		return errors.New("rate limit must be non-negative")
	}
	if c.MaxDepth < 0 {
		return errors.New("max depth must be non-negative")
	}
	if c.MaxConcurrency < 1 {
		return errors.New("max concurrency must be positive")
	}
	return nil
}

// setupCollector configures the collector with the given source settings.
func (c *Crawler) setupCollector(source *configtypes.Source) error {
	// Use override if set, otherwise use source's max depth
	maxDepth := source.MaxDepth
	override := int(atomic.LoadInt32(&c.maxDepthOverride))
	if override > 0 {
		maxDepth = override
		c.logger.Info("Using max_depth override", "override", maxDepth, "source_default", source.MaxDepth)
	}

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

	err = c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       rateLimit,
		RandomDelay: rateLimit / RandomDelayDivisor,
		Parallelism: defaultParallelism,
	})
	if err != nil {
		return fmt.Errorf("failed to set rate limit: %w", err)
	}

	// Configure transport with TLS settings from config
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

// SetMaxDepth sets the maximum depth for the crawler.
// If the collector hasn't been created yet, this sets an override that will be used
// when the collector is created. Otherwise, it updates the existing collector.
func (c *Crawler) SetMaxDepth(depth int) {
	config := NewCollectorConfig()
	config.MaxDepth = depth

	if err := config.Validate(); err != nil {
		c.logger.Error("Invalid max depth",
			"error", err,
			"depth", depth)
		return
	}

	// Always store the override so it's used when setupCollector creates a new collector
	// Bounds check to prevent integer overflow
	maxDepth := config.MaxDepth
	if maxDepth >= math.MinInt32 && maxDepth <= math.MaxInt32 {
		atomic.StoreInt32(&c.maxDepthOverride, int32(maxDepth))
	}

	if c.collector == nil {
		// Collector not created yet, override will be used when collector is created
		c.logger.Debug("Set max_depth override (collector not yet created)", "max_depth", config.MaxDepth)
	} else {
		// Collector exists, update it directly
		c.collector.MaxDepth = config.MaxDepth
		c.logger.Debug("Updated collector max_depth", "max_depth", config.MaxDepth)
	}
}

// SetRateLimit sets the rate limit for the crawler.
func (c *Crawler) SetRateLimit(duration time.Duration) error {
	if c.collector == nil {
		return errors.New("collector is nil")
	}

	config := NewCollectorConfig()
	config.RateLimit = duration

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid rate limit: %w", err)
	}

	err := c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       config.RateLimit,
		RandomDelay: 0,
		Parallelism: config.MaxConcurrency,
	})
	if err != nil {
		return fmt.Errorf("failed to set rate limit: %w", err)
	}

	return nil
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

const (
	// RandomDelayDivisor is used to calculate random delay from rate limit
	RandomDelayDivisor = 2
)

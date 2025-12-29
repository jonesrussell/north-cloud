// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

const (
	// DefaultChannelBufferSize is the default buffer size for processor channels.
	DefaultChannelBufferSize = 100
	// DefaultMaxIdleConns is the default maximum number of idle connections.
	DefaultMaxIdleConns = 100
	// DefaultMaxIdleConnsPerHost is the default maximum number of idle connections per host.
	DefaultMaxIdleConnsPerHost = 10
	// DefaultIdleConnTimeout is the default idle connection timeout.
	DefaultIdleConnTimeout = 90 * time.Second
	// DefaultResponseHeaderTimeout is the default response header timeout.
	DefaultResponseHeaderTimeout = 30 * time.Second
	// DefaultExpectContinueTimeout is the default expect continue timeout.
	DefaultExpectContinueTimeout = 1 * time.Second
)

// CrawlerParams holds parameters for creating a crawler instance
type CrawlerParams struct {
	Logger       logger.Interface
	Bus          *events.EventBus
	IndexManager types.IndexManager
	Sources      sources.Interface
	Config       *crawler.Config
	Storage      types.Interface
	FullConfig   config.Interface // Full config for accessing MinIO settings
	DB           any              // Database connection (optional, for queued links)
}

// CrawlerResult holds the crawler instance
type CrawlerResult struct {
	Crawler Interface
}

// createCollector creates and configures a new colly collector
func createCollector(cfg *crawler.Config, log logger.Interface) (*colly.Collector, error) {
	collector := colly.NewCollector(
		colly.MaxDepth(cfg.MaxDepth),
		colly.Async(true),
		colly.AllowedDomains(cfg.AllowedDomains...),
		colly.ParseHTTPErrorResponse(),
		colly.IgnoreRobotsTxt(),
		colly.UserAgent(cfg.UserAgent),
		// Note: Not using AllowURLRevisit() to prevent excessive request queuing.
		// Each URL will only be crawled once, which significantly reduces Wait() time.
	)

	// Configure rate limiting
	if err := collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       cfg.Delay,
		RandomDelay: cfg.RandomDelay,
		Parallelism: cfg.MaxConcurrency,
	}); err != nil {
		return nil, fmt.Errorf("failed to set rate limit: %w", err)
	}

	// Configure transport with TLS settings from config
	collector.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify, //nolint:gosec // Configurable for development/testing
			MinVersion:         cfg.TLS.MinVersion,
			MaxVersion:         cfg.TLS.MaxVersion,
		},
		DisableKeepAlives:     false,
		MaxIdleConns:          DefaultMaxIdleConns,
		MaxIdleConnsPerHost:   DefaultMaxIdleConnsPerHost,
		IdleConnTimeout:       DefaultIdleConnTimeout,
		ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
		ExpectContinueTimeout: DefaultExpectContinueTimeout,
	})

	if cfg.TLS.InsecureSkipVerify {
		log.Warn("TLS certificate verification is disabled. This is not recommended for production use.",
			"component", "crawler",
			"warning", "This makes HTTPS connections vulnerable to man-in-the-middle attacks")
	}

	// Set up callbacks
	collector.OnRequest(func(r *colly.Request) {
		log.Info("Visiting", "url", r.URL.String())
	})

	collector.OnResponse(func(r *colly.Response) {
		log.Info("Visited", "url", r.Request.URL.String(), "status", r.StatusCode)
	})

	collector.OnError(func(r *colly.Response, err error) {
		log.Error("Error while crawling",
			"url", r.Request.URL.String(),
			"status", r.StatusCode,
			"error", err)
	})

	return collector, nil
}

// NewCrawlerWithParams creates a new crawler instance with all its components.
// This is the non-FX version that replaces ProvideCrawler.
func NewCrawlerWithParams(p CrawlerParams) (*CrawlerResult, error) {
	// Create raw content service and processor (primary processor for all content)
	rawContentService := rawcontent.NewRawContentService(
		p.Logger,
		p.Storage,
		p.Sources,
	)
	rawContentProcessor := rawcontent.NewProcessor(
		p.Logger,
		rawContentService,
	)

	// Create collector
	collector, err := createCollector(p.Config, p.Logger)
	if err != nil {
		return nil, err
	}

	// Create lifecycle and signal coordinators
	lifecycle := NewLifecycleManager()
	signals := NewSignalCoordinator(p.Config, p.Logger)

	// Initialize HTML archiver if MinIO is configured
	var archiver Archiver
	if p.FullConfig != nil {
		minioConfig := p.FullConfig.GetMinIOConfig()
		if minioConfig != nil && minioConfig.Enabled {
			arch, archErr := archive.NewArchiver(minioConfig, p.Logger)
			if archErr != nil {
				p.Logger.Warn("Failed to initialize MinIO archiver, continuing without archiving",
					"error", archErr)
			} else {
				archiver = arch
				p.Logger.Info("MinIO archiver initialized successfully")
			}
		}
	}

	// Create crawler
	c := &Crawler{
		logger:              p.Logger,
		collector:           collector,
		bus:                 p.Bus,
		indexManager:        p.IndexManager,
		sources:             p.Sources,
		rawContentProcessor: rawContentProcessor,
		state:               NewState(p.Logger),
		processors:          []content.Processor{rawContentProcessor},
		htmlProcessor:       NewHTMLProcessor(p.Logger, p.Sources),
		cfg:                 p.Config,
		lifecycle:           lifecycle,
		signals:             signals,
		archiver:            archiver,
	}

	// Create queued link repository if DB is available
	linkRepo := createQueuedLinkRepository(p)

	// Create link handler with repository and save links flag
	c.linkHandler = NewLinkHandler(c, linkRepo, p.Config.SaveDiscoveredLinks)

	return &CrawlerResult{
		Crawler: c,
	}, nil
}

// createQueuedLinkRepository creates a queued link repository if DB is available.
func createQueuedLinkRepository(p CrawlerParams) *database.QueuedLinkRepository {
	if p.DB == nil {
		if p.Config.SaveDiscoveredLinks {
			p.Logger.Warn(
				"SaveDiscoveredLinks is enabled but no database connection available - " +
					"queued link saving will be disabled")
		}
		return nil
	}

	// Type assert to *sqlx.DB
	db, ok := p.DB.(*sqlx.DB)
	if !ok {
		p.Logger.Warn("Database connection type assertion failed - queued link saving will be disabled")
		return nil
	}

	linkRepo := database.NewQueuedLinkRepository(db)
	if p.Config.SaveDiscoveredLinks {
		p.Logger.Info("Queued link saving enabled - discovered links will be saved to database")
	} else {
		p.Logger.Debug("Queued link saving disabled - set CRAWLER_SAVE_DISCOVERED_LINKS=true to enable")
	}

	return linkRepo
}

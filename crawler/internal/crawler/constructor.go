// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
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
	Logger       infralogger.Logger
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
					infralogger.Error(archErr))
			} else {
				archiver = arch
				p.Logger.Info("MinIO archiver initialized successfully")
			}
		}
	}

	// Create crawler
	c := &Crawler{
		logger:              p.Logger,
		collector:           nil, // Collector will be created in setupCollector() when Start() is called
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

	// Create discovered link repository if DB is available
	linkRepo := createDiscoveredLinkRepository(p)

	// Create link handler with repository and save links flag
	c.linkHandler = NewLinkHandler(c, linkRepo, p.Config.SaveDiscoveredLinks)

	return &CrawlerResult{
		Crawler: c,
	}, nil
}

// createDiscoveredLinkRepository creates a discovered link repository if DB is available.
func createDiscoveredLinkRepository(p CrawlerParams) *database.DiscoveredLinkRepository {
	if p.DB == nil {
		if p.Config.SaveDiscoveredLinks {
			p.Logger.Warn(
				"SaveDiscoveredLinks is enabled but no database connection available - " +
					"discovered link saving will be disabled")
		}
		return nil
	}

	// Type assert to *sqlx.DB
	db, ok := p.DB.(*sqlx.DB)
	if !ok {
		p.Logger.Warn("Database connection type assertion failed - discovered link saving will be disabled")
		return nil
	}

	linkRepo := database.NewDiscoveredLinkRepository(db)
	if p.Config.SaveDiscoveredLinks {
		p.Logger.Info("Discovered link saving enabled - discovered links will be saved to database")
	} else {
		p.Logger.Debug("Discovered link saving disabled - set CRAWLER_SAVE_DISCOVERED_LINKS=true to enable")
	}

	return linkRepo
}

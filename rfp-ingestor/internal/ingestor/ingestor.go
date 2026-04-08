package ingestor

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/config"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
	esindex "github.com/jonesrussell/north-cloud/rfp-ingestor/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/feed"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/parser"
)

// Config holds the settings needed to run a single ingestion cycle.
type Config struct {
	FeedURL  string
	ESURL    string
	ESIndex  string
	BulkSize int
}

// RunResult summarises the outcome of a single ingestion cycle.
type RunResult struct {
	Fetched  int
	Indexed  int
	Failed   int
	Duration time.Duration
}

// Ingestor orchestrates the fetch -> parse -> index pipeline.
type Ingestor struct {
	cfg     Config
	fetcher *feed.Fetcher
	parsers map[string]parser.PortalParser
	sources []config.FeedSource
	log     logger.Logger
}

// NewIngestor creates an Ingestor with a new HTTP feed fetcher.
// When sources is nil, it falls back to a single-URL legacy mode using cfg.FeedURL.
func NewIngestor(cfg Config, log logger.Logger, opts ...Option) *Ingestor {
	ing := &Ingestor{
		cfg:     cfg,
		fetcher: feed.NewFetcher(),
		parsers: make(map[string]parser.PortalParser),
		log:     log,
	}

	for _, opt := range opts {
		opt(ing)
	}

	return ing
}

// Option configures an Ingestor.
type Option func(*Ingestor)

// WithParsers sets the parser registry.
func WithParsers(parsers map[string]parser.PortalParser) Option {
	return func(ing *Ingestor) {
		ing.parsers = parsers
	}
}

// WithSources sets the feed sources to poll.
func WithSources(sources []config.FeedSource) Option {
	return func(ing *Ingestor) {
		ing.sources = sources
	}
}

// RunOnce executes a single ingestion cycle: fetch feeds, parse rows, bulk-index
// into Elasticsearch. It returns a summary of the run and any fatal error that
// prevented completion.
func (ing *Ingestor) RunOnce(ctx context.Context) (RunResult, error) {
	start := time.Now()

	// Multi-source mode: iterate configured sources.
	if len(ing.sources) > 0 {
		return ing.runMultiSource(ctx, start)
	}

	// Legacy single-URL mode (backward compatible).
	return ing.runSingleURL(ctx, start)
}

func (ing *Ingestor) runMultiSource(ctx context.Context, start time.Time) (RunResult, error) {
	var totalResult RunResult

	for _, source := range ing.sources {
		p, ok := ing.parsers[source.Parser]
		if !ok {
			ing.log.Warn("no parser registered for source",
				logger.String("source", source.Name),
				logger.String("parser", source.Parser),
			)
			totalResult.Failed++
			continue
		}

		for _, url := range source.URLs {
			result, err := ing.fetchParseIndex(ctx, url, p)
			if err != nil {
				ing.log.Error("source ingestion failed",
					logger.String("source", source.Name),
					logger.String("url", url),
					logger.Error(err),
				)
				totalResult.Failed++
				continue
			}

			totalResult.Fetched += result.Fetched
			totalResult.Indexed += result.Indexed
			totalResult.Failed += result.Failed
		}
	}

	totalResult.Duration = time.Since(start)
	return totalResult, nil
}

func (ing *Ingestor) runSingleURL(ctx context.Context, start time.Time) (RunResult, error) {
	// 1. Fetch the CSV feed.
	body, modified, err := ing.fetcher.Fetch(ctx, ing.cfg.FeedURL)
	if err != nil {
		return RunResult{Duration: time.Since(start)}, fmt.Errorf("fetch feed: %w", err)
	}
	if !modified {
		return RunResult{Duration: time.Since(start)}, nil
	}
	defer body.Close()

	// 2. Parse CSV rows into RFP documents.
	docs, parseErrs := ParseCSV(body)
	result := RunResult{Fetched: len(docs), Failed: len(parseErrs)}

	for _, e := range parseErrs {
		ing.log.Warn("parse warning", logger.Error(e))
	}

	if len(docs) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// 3. Build ID -> document map for bulk indexing.
	docMap := make(map[string]domain.RFPDocument, len(docs))
	for i := range docs {
		docMap[DocumentID(docs[i])] = docs[i]
	}

	// 4. Bulk-index documents into Elasticsearch.
	indexed, failed, indexErr := ing.bulkIndex(ctx, docMap)
	result.Indexed = indexed
	result.Failed += failed
	result.Duration = time.Since(start)

	if indexErr != nil {
		return result, indexErr
	}

	return result, nil
}

// fetchParseIndex handles a single URL: fetch, parse with the given parser, and bulk-index.
func (ing *Ingestor) fetchParseIndex(ctx context.Context, url string, p parser.PortalParser) (RunResult, error) {
	body, modified, err := ing.fetcher.Fetch(ctx, url)
	if err != nil {
		return RunResult{}, fmt.Errorf("fetch %s: %w", url, err)
	}
	if !modified {
		return RunResult{}, nil
	}
	defer body.Close()

	docMap, err := p.Parse(body)
	if err != nil {
		return RunResult{}, fmt.Errorf("parse %s: %w", p.SourceName(), err)
	}

	result := RunResult{Fetched: len(docMap)}

	if len(docMap) == 0 {
		return result, nil
	}

	indexed, failed, indexErr := ing.bulkIndex(ctx, docMap)
	result.Indexed = indexed
	result.Failed = failed

	return result, indexErr
}

// bulkIndex sends documents to Elasticsearch.
func (ing *Ingestor) bulkIndex(ctx context.Context, docMap map[string]domain.RFPDocument) (indexed, failed int, err error) {
	indexer, createErr := esindex.NewIndexer(ing.cfg.ESURL, ing.cfg.ESIndex, ing.cfg.BulkSize)
	if createErr != nil {
		return 0, 0, fmt.Errorf("create indexer: %w", createErr)
	}

	bulkResult, bulkErr := indexer.BulkIndex(ctx, docMap)
	if bulkErr != nil {
		return 0, 0, fmt.Errorf("bulk index: %w", bulkErr)
	}

	for _, e := range bulkResult.Errors {
		ing.log.Warn("bulk index error", logger.String("error", e))
	}

	return bulkResult.Indexed, bulkResult.Failed, nil
}

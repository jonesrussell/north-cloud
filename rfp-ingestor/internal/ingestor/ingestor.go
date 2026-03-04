package ingestor

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
	esindex "github.com/jonesrussell/north-cloud/rfp-ingestor/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/feed"
	"github.com/north-cloud/infrastructure/logger"
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
	Skipped  int
	Failed   int
	Duration time.Duration
}

// Ingestor orchestrates the fetch -> parse -> index pipeline.
type Ingestor struct {
	cfg     Config
	fetcher *feed.Fetcher
	log     logger.Logger
}

// NewIngestor creates an Ingestor with a new HTTP feed fetcher.
func NewIngestor(cfg Config, log logger.Logger) *Ingestor {
	return &Ingestor{
		cfg:     cfg,
		fetcher: feed.NewFetcher(),
		log:     log,
	}
}

// RunOnce executes a single ingestion cycle: fetch CSV, parse rows, bulk-index
// into Elasticsearch. It returns a summary of the run and any fatal error that
// prevented completion.
func (ing *Ingestor) RunOnce(ctx context.Context) (RunResult, error) {
	start := time.Now()

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
	for _, doc := range docs {
		docMap[DocumentID(doc)] = doc
	}

	// 4. Bulk-index documents into Elasticsearch.
	indexer, err := esindex.NewIndexer(ing.cfg.ESURL, ing.cfg.ESIndex, ing.cfg.BulkSize)
	if err != nil {
		result.Duration = time.Since(start)
		return result, fmt.Errorf("create indexer: %w", err)
	}

	bulkResult, err := indexer.BulkIndex(ctx, docMap)
	if err != nil {
		result.Duration = time.Since(start)
		return result, fmt.Errorf("bulk index: %w", err)
	}

	result.Indexed = bulkResult.Indexed
	result.Failed += bulkResult.Failed
	result.Duration = time.Since(start)

	return result, nil
}

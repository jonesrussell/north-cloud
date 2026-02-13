package service

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
)

// AggregationESClient defines the Elasticsearch operations needed by AggregationService.
// The concrete *elasticsearch.Client satisfies this interface.
type AggregationESClient interface {
	SearchAllClassifiedContent(ctx context.Context, query map[string]any) (*esapi.Response, error)
	GetAllIndexDocCounts(ctx context.Context) ([]elasticsearch.IndexDocCount, error)
}

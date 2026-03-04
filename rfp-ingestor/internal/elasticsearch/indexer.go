package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// BulkResult summarises the outcome of a bulk indexing operation.
type BulkResult struct {
	Indexed int
	Failed  int
	Errors  []string
}

// Indexer sends RFP documents to Elasticsearch using the _bulk API.
type Indexer struct {
	client    *http.Client
	baseURL   string
	indexName string
	bulkSize  int
}

// NewIndexer creates an Indexer that targets esURL/{indexName}.
// Returns an error when esURL or indexName is empty, or bulkSize is not positive.
func NewIndexer(esURL, indexName string, bulkSize int) (*Indexer, error) {
	if esURL == "" {
		return nil, fmt.Errorf("elasticsearch URL must not be empty")
	}
	if indexName == "" {
		return nil, fmt.Errorf("index name must not be empty")
	}
	if bulkSize <= 0 {
		return nil, fmt.Errorf("bulk size must be positive, got %d", bulkSize)
	}

	return &Indexer{
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   strings.TrimRight(esURL, "/"),
		indexName: indexName,
		bulkSize:  bulkSize,
	}, nil
}

// bulkAction is the envelope Elasticsearch expects on every odd NDJSON line.
type bulkAction struct {
	Index bulkActionMeta `json:"index"`
}

// bulkActionMeta carries the per-document routing fields.
type bulkActionMeta struct {
	Index string `json:"_index"`
	ID    string `json:"_id"`
}

// bulkResponseItem mirrors just enough of the ES _bulk response to detect errors.
type bulkResponseItem struct {
	Index struct {
		Status int              `json:"status"`
		Error  *json.RawMessage `json:"error,omitempty"`
	} `json:"index"`
}

// bulkResponse is the top-level _bulk response body.
type bulkResponse struct {
	Errors bool               `json:"errors"`
	Items  []bulkResponseItem `json:"items"`
}

// BulkIndex sends every document in docs to Elasticsearch, batched by bulkSize.
// The map key is used as the document _id.
func (ix *Indexer) BulkIndex(ctx context.Context, docs map[string]domain.RFPDocument) (BulkResult, error) {
	if len(docs) == 0 {
		return BulkResult{}, nil
	}

	// Collect keys so we can iterate in batches of bulkSize.
	keys := make([]string, 0, len(docs))
	for k := range docs {
		keys = append(keys, k)
	}

	var total BulkResult

	for start := 0; start < len(keys); start += ix.bulkSize {
		end := start + ix.bulkSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[start:end]

		body, buildErr := ix.buildBulkBody(batch, docs)
		if buildErr != nil {
			return total, fmt.Errorf("build bulk body: %w", buildErr)
		}

		result, sendErr := ix.sendBulk(ctx, body)
		if sendErr != nil {
			return total, fmt.Errorf("send bulk request: %w", sendErr)
		}

		total.Indexed += result.Indexed
		total.Failed += result.Failed
		total.Errors = append(total.Errors, result.Errors...)
	}

	return total, nil
}

// buildBulkBody produces an NDJSON payload for the given batch of document keys.
func (ix *Indexer) buildBulkBody(keys []string, docs map[string]domain.RFPDocument) ([]byte, error) {
	var buf bytes.Buffer

	for _, id := range keys {
		action := bulkAction{
			Index: bulkActionMeta{
				Index: ix.indexName,
				ID:    id,
			},
		}

		actionLine, err := json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("marshal action for %s: %w", id, err)
		}
		buf.Write(actionLine)
		buf.WriteByte('\n')

		docLine, err := json.Marshal(docs[id])
		if err != nil {
			return nil, fmt.Errorf("marshal document %s: %w", id, err)
		}
		buf.Write(docLine)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

// sendBulk POSTs an NDJSON payload to the _bulk endpoint and parses the response.
func (ix *Indexer) sendBulk(ctx context.Context, body []byte) (BulkResult, error) {
	url := ix.baseURL + "/_bulk"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return BulkResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := ix.client.Do(req)
	if err != nil {
		return BulkResult{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return BulkResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return BulkResult{}, fmt.Errorf("bulk request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var bulkResp bulkResponse
	if err := json.Unmarshal(respBody, &bulkResp); err != nil {
		return BulkResult{}, fmt.Errorf("parse bulk response: %w", err)
	}

	return tallyResults(bulkResp), nil
}

// tallyResults walks the _bulk response items and counts successes vs failures.
func tallyResults(resp bulkResponse) BulkResult {
	var result BulkResult

	for _, item := range resp.Items {
		if item.Index.Status >= 200 && item.Index.Status < 300 {
			result.Indexed++
			continue
		}

		result.Failed++
		if item.Index.Error != nil {
			result.Errors = append(result.Errors, string(*item.Index.Error))
		}
	}

	return result
}

// RecreateIndex deletes and recreates the index with the supplied mapping.
// Use this when the mapping has changed and the old index must be replaced.
func (ix *Indexer) RecreateIndex(ctx context.Context, mapping map[string]any) error {
	url := ix.baseURL + "/" + ix.indexName

	delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create DELETE request: %w", err)
	}

	delResp, err := ix.client.Do(delReq)
	if err != nil {
		return fmt.Errorf("delete index: %w", err)
	}
	delResp.Body.Close()
	// Ignore 404 (index didn't exist)

	return ix.EnsureIndex(ctx, mapping)
}

// EnsureIndex creates the index with the supplied mapping if it does not already exist.
func (ix *Indexer) EnsureIndex(ctx context.Context, mapping map[string]any) error {
	url := ix.baseURL + "/" + ix.indexName

	// Check whether the index already exists.
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("create HEAD request: %w", err)
	}

	headResp, err := ix.client.Do(headReq)
	if err != nil {
		return fmt.Errorf("check index existence: %w", err)
	}
	headResp.Body.Close()

	if headResp.StatusCode == http.StatusOK {
		return nil
	}
	if headResp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status %d checking index existence", headResp.StatusCode)
	}

	// Index does not exist (404); create it.
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(mappingJSON))
	if err != nil {
		return fmt.Errorf("create PUT request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := ix.client.Do(putReq)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(putResp.Body)
		if readErr != nil {
			return fmt.Errorf("create index failed with status %d (body unreadable: %w)", putResp.StatusCode, readErr)
		}
		return fmt.Errorf("create index failed with status %d: %s", putResp.StatusCode, string(respBody))
	}

	return nil
}

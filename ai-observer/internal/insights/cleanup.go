package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	es "github.com/elastic/go-elasticsearch/v8"
)

// Cleaner deletes insights older than a configured retention period.
type Cleaner struct {
	esClient      *es.Client
	retentionDays int
}

// NewCleaner creates a Cleaner that deletes insights older than retentionDays.
func NewCleaner(esClient *es.Client, retentionDays int) *Cleaner {
	return &Cleaner{esClient: esClient, retentionDays: retentionDays}
}

// DeleteOld removes all documents from ai_insights older than the retention window.
// Returns the number of deleted documents.
func (c *Cleaner) DeleteOld(ctx context.Context) (int, error) {
	if c.retentionDays <= 0 {
		return 0, nil
	}

	query := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"created_at": map[string]any{
					"lt": fmt.Sprintf("now-%dd", c.retentionDays),
				},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("marshal cleanup query: %w", err)
	}

	res, err := c.esClient.DeleteByQuery(
		[]string{insightsIndex},
		bytes.NewReader(body),
		c.esClient.DeleteByQuery.WithContext(ctx),
	)
	if err != nil {
		return 0, fmt.Errorf("delete by query: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return 0, fmt.Errorf("delete by query error: %s", res.String())
	}

	return parseDeletedCount(res.Body)
}

// deleteByQueryResponse is the minimal shape of the ES delete_by_query response.
type deleteByQueryResponse struct {
	Deleted int `json:"deleted"`
}

// parseDeletedCount extracts the deleted count from an ES delete_by_query response.
func parseDeletedCount(r io.Reader) (int, error) {
	var resp deleteByQueryResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return 0, fmt.Errorf("decode cleanup response: %w", err)
	}

	return resp.Deleted, nil
}

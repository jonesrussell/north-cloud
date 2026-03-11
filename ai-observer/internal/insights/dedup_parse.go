package insights

import (
	"encoding/json"
	"fmt"
	"io"
)

// esAggResponse is the minimal ES aggregation response shape for dedup queries.
type esAggResponse struct {
	Aggregations struct {
		RecentSummaries struct {
			Buckets []struct {
				Key string `json:"key"`
			} `json:"buckets"`
		} `json:"recent_summaries"`
	} `json:"aggregations"`
}

// parseSummaryBuckets extracts unique summary strings from an ES aggregation response.
func parseSummaryBuckets(r io.Reader) (map[string]bool, error) {
	var resp esAggResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode dedup response: %w", err)
	}

	buckets := resp.Aggregations.RecentSummaries.Buckets
	result := make(map[string]bool, len(buckets))
	for _, b := range buckets {
		result[b.Key] = true
	}

	return result, nil
}

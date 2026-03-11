package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultDocLimit = 20
)

// buildQuery constructs an Elasticsearch query filtering by source_name,
// sorted by indexed_at descending, with the given result limit.
func buildQuery(source string, limit int) map[string]any {
	return map[string]any{
		"size": limit,
		"query": map[string]any{
			"term": map[string]any{
				"source_name.keyword": source,
			},
		},
		"sort": []map[string]any{
			{"indexed_at": map[string]any{"order": "desc"}},
		},
	}
}

// fetchDocuments queries Elasticsearch and returns the hit documents.
func fetchDocuments(esURL, index, source string, limit int) ([]map[string]any, error) {
	query := buildQuery(source, limit)

	body, marshalErr := json.Marshal(query)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshaling query: %w", marshalErr)
	}

	url := fmt.Sprintf("%s/%s/_search", esURL, index)

	req, reqErr := http.NewRequest(http.MethodGet, url, bytes.NewReader(body))
	if reqErr != nil {
		return nil, fmt.Errorf("creating request: %w", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, httpErr := http.DefaultClient.Do(req)
	if httpErr != nil {
		return nil, fmt.Errorf("executing request: %w", httpErr)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("reading response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ES returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if unmarshalErr := json.Unmarshal(respBody, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", unmarshalErr)
	}

	docs := make([]map[string]any, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs, nil
}

// countWords returns the number of whitespace-delimited words in the text.
func countWords(text string) int {
	fields := strings.Fields(text)
	return len(fields)
}

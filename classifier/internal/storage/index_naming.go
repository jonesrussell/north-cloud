package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// validIndexChar matches characters allowed in ES index names: lowercase alphanumeric and underscore.
var validIndexChar = regexp.MustCompile(`[^a-z0-9_]`)

// collapseUnderscores replaces runs of multiple underscores with a single one.
var collapseUnderscores = regexp.MustCompile(`_+`)

const (
	rawContentSuffix        = "_raw_content"
	classifiedContentSuffix = "_classified_content"
)

// GetClassifiedIndexName returns the classified_content index name for a raw_content index.
func GetClassifiedIndexName(rawIndex string) (string, error) {
	if !strings.HasSuffix(rawIndex, rawContentSuffix) {
		return "", errors.New("invalid raw_content index name")
	}
	return rawIndex[:len(rawIndex)-len(rawContentSuffix)] + classifiedContentSuffix, nil
}

// SanitizeSourceName converts a human-readable source name into a valid ES index prefix.
// Lowercases, replaces non-alphanumeric chars with underscore, collapses runs, trims.
func SanitizeSourceName(name string) string {
	if name == "" {
		return ""
	}
	s := strings.ToLower(strings.TrimSpace(name))
	s = validIndexChar.ReplaceAllString(s, "_")
	s = collapseUnderscores.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

// ClassifiedIndexForContent determines the classified index name for a content item.
// Prefers SourceIndex (derived from ES _index field); falls back to sanitized SourceName.
func ClassifiedIndexForContent(sourceIndex, sourceName string) (string, error) {
	if sourceIndex != "" {
		return GetClassifiedIndexName(sourceIndex)
	}
	sanitized := SanitizeSourceName(sourceName)
	if sanitized == "" {
		return "", errors.New("cannot determine classified index: both source_index and source_name are empty")
	}
	return sanitized + classifiedContentSuffix, nil
}

// bulkResponse is the minimal structure needed to check for item-level errors.
type bulkResponse struct {
	Errors bool       `json:"errors"`
	Items  []bulkItem `json:"items"`
}

// bulkItem represents one action result in a bulk response.
type bulkItem struct {
	Index bulkItemResult `json:"index"`
}

// bulkItemResult holds the status and optional error for a single bulk item.
type bulkItemResult struct {
	Index  string         `json:"_index"`
	ID     string         `json:"_id"`
	Status int            `json:"status"`
	Error  map[string]any `json:"error,omitempty"`
}

// checkBulkResponse parses an ES bulk response body and returns an error if any items failed.
func checkBulkResponse(body []byte) error {
	var resp bulkResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	if !resp.Errors {
		return nil
	}

	var failedCount int
	var firstErr string
	for _, item := range resp.Items {
		if item.Index.Error != nil {
			failedCount++
			if firstErr == "" {
				reason, _ := item.Index.Error["reason"].(string)
				errType, _ := item.Index.Error["type"].(string)
				firstErr = fmt.Sprintf("index=%s id=%s type=%s reason=%s",
					item.Index.Index, item.Index.ID, errType, reason)
			}
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d of %d bulk items failed; first error: %s", failedCount, len(resp.Items), firstErr)
	}

	return nil
}

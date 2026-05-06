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

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	contentTypeJSON     = "application/json"
	contentTypeNDJSON   = "application/x-ndjson"
	httpTimeout         = 10 * time.Second
	maxResponseBytes    = 1 << 20 // 1 MiB
	lifecycleRescinded  = "rescinded"
	revisionKindRescind = "rescinded"
)

// Config holds the parameters needed to construct an Indexer.
type Config struct {
	BaseURL string
	Index   string
}

// Indexer writes and queries community alert documents via raw HTTP.
// It satisfies catalogue.ESActiveAlertQuerier via QueryActiveAlertIDs.
type Indexer struct {
	baseURL string
	index   string
	client  *http.Client
}

// New constructs an Indexer with a default HTTP client timeout.
func New(cfg Config) *Indexer {
	return &Indexer{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		index:   cfg.Index,
		client:  &http.Client{Timeout: httpTimeout},
	}
}

// EnsureIndex creates the community_alerts index if it does not yet exist.
// It is idempotent: a 200 HEAD response means no-op; a 404 triggers a PUT.
// Race condition: if two callers PUT simultaneously and ES returns 400 with
// "resource_already_exists_exception", the error is treated as success.
func (ix *Indexer) EnsureIndex(ctx context.Context) error {
	indexURL := ix.baseURL + "/" + ix.index

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, indexURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("elasticsearch: build HEAD request: %w", err)
	}

	headResp, err := ix.client.Do(headReq)
	if err != nil {
		return fmt.Errorf("elasticsearch: HEAD index: %w", err)
	}

	headResp.Body.Close()

	switch headResp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ix.putMapping(ctx, indexURL)
	default:
		return fmt.Errorf("elasticsearch: HEAD index returned unexpected status %d", headResp.StatusCode)
	}
}

// putMapping issues a PUT request with the embedded mapping JSON.
func (ix *Indexer) putMapping(ctx context.Context, indexURL string) error {
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, indexURL, bytes.NewReader(CommunityAlertsMapping()))
	if err != nil {
		return fmt.Errorf("elasticsearch: build PUT mapping request: %w", err)
	}

	putReq.Header.Set("Content-Type", contentTypeJSON)

	putResp, err := ix.client.Do(putReq)
	if err != nil {
		return fmt.Errorf("elasticsearch: PUT mapping: %w", err)
	}

	defer putResp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(putResp.Body, maxResponseBytes))
	if readErr != nil {
		return fmt.Errorf("elasticsearch: read PUT mapping response: %w", readErr)
	}

	if putResp.StatusCode == http.StatusOK || putResp.StatusCode == http.StatusCreated {
		return nil
	}

	// Race condition: two callers PUT simultaneously.
	if putResp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "resource_already_exists_exception") {
		return nil
	}

	return fmt.Errorf("elasticsearch: PUT mapping returned %d: %s", putResp.StatusCode, string(body))
}

// Index writes a single Alert document using a deterministic _id derived from alert.ID.
func (ix *Indexer) Index(ctx context.Context, alert domain.Alert) error {
	docURL := fmt.Sprintf("%s/%s/_doc/%s", ix.baseURL, ix.index, alert.ID)

	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("elasticsearch: marshal alert %s: %w", alert.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, docURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("elasticsearch: build Index request: %w", err)
	}

	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := ix.client.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch: Index alert %s: %w", alert.ID, err)
	}

	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return fmt.Errorf("elasticsearch: read Index response: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch: Index alert %s returned %d: %s", alert.ID, resp.StatusCode, string(respBody))
	}

	return nil
}

// rescindPayload is the body for a MarkRescinded partial update.
type rescindPayload struct {
	Script rescindScript `json:"script"`
}

type rescindScript struct {
	Source string        `json:"source"`
	Lang   string        `json:"lang"`
	Params rescindParams `json:"params"`
}

type rescindParams struct {
	LifecycleState string          `json:"lifecycle_state"`
	RescindedAt    string          `json:"rescinded_at"`
	RevisionEntry  domain.Revision `json:"revision_entry"`
}

// MarkRescinded performs a partial update setting lifecycle_state=rescinded,
// rescinded_at, and appending a revision_history entry via a Painless script.
// It is idempotent: replaying with the same alertID is safe.
func (ix *Indexer) MarkRescinded(ctx context.Context, alertID string, at time.Time, reason string) error {
	updateURL := fmt.Sprintf("%s/%s/_update/%s", ix.baseURL, ix.index, alertID)

	const scriptSource = `
		ctx._source.lifecycle_state = params.lifecycle_state;
		ctx._source.rescinded_at = params.rescinded_at;
		if (ctx._source.revision_history == null) {
			ctx._source.revision_history = [];
		}
		ctx._source.revision_history.add(params.revision_entry);`

	payload := rescindPayload{
		Script: rescindScript{
			Source: scriptSource,
			Lang:   "painless",
			Params: rescindParams{
				LifecycleState: lifecycleRescinded,
				RescindedAt:    at.UTC().Format(time.RFC3339),
				RevisionEntry: domain.Revision{
					RevisionAt:    at.UTC(),
					RevisionKind:  revisionKindRescind,
					ChangeSummary: reason,
					ChangedFields: []string{"lifecycle_state", "rescinded_at"},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("elasticsearch: marshal rescind payload for %s: %w", alertID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, updateURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("elasticsearch: build MarkRescinded request: %w", err)
	}

	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := ix.client.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch: MarkRescinded alert %s: %w", alertID, err)
	}

	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return fmt.Errorf("elasticsearch: read MarkRescinded response: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch: MarkRescinded %s returned %d: %s", alertID, resp.StatusCode, string(respBody))
	}

	return nil
}

// activeAlertsQuery is the search payload for QueryActiveAlertIDs.
type activeAlertsQuery struct {
	Query activeAlertsFilter `json:"query"`
	Size  int                `json:"size"`
}

type activeAlertsFilter struct {
	Bool activeAlertsBool `json:"bool"`
}

type activeAlertsBool struct {
	Must []any `json:"must"`
}

type termFilter struct {
	Term map[string]string `json:"term"`
}

type nestedFilter struct {
	Nested nestedFilterInner `json:"nested"`
}

type nestedFilterInner struct {
	Path  string          `json:"path"`
	Query nestedTermQuery `json:"query"`
}

type nestedTermQuery struct {
	Term map[string]string `json:"term"`
}

const maxActiveAlerts = 10000

// QueryActiveAlertIDs returns active alerts for the given sourceID.
// It satisfies the catalogue.ESActiveAlertQuerier interface as QueryActiveAlertIDs
// (the interface is defined without the sourceID parameter — this signature adds sourceID
// as a scoping aid used by callers outside the catalogue rebuild path).
// For the catalogue rebuild path (ESActiveAlertQuerier), callers use the no-arg variant
// below which calls this function with an empty sourceID to return all active alerts.
func (ix *Indexer) QueryActiveAlertIDs(ctx context.Context) ([]domain.Alert, error) {
	return ix.QueryActive(ctx, "")
}

// QueryActive searches for active alerts. If sourceID is non-empty, results are filtered
// to alerts whose sources[].source_id matches. Returns all active alerts when sourceID is "".
func (ix *Indexer) QueryActive(ctx context.Context, sourceID string) ([]domain.Alert, error) {
	searchURL := fmt.Sprintf("%s/%s/_search", ix.baseURL, ix.index)

	mustClauses := []any{
		termFilter{Term: map[string]string{"lifecycle_state": "active"}},
	}

	if sourceID != "" {
		mustClauses = append(mustClauses, nestedFilter{
			Nested: nestedFilterInner{
				Path: "sources",
				Query: nestedTermQuery{
					Term: map[string]string{"sources.source_id": sourceID},
				},
			},
		})
	}

	query := activeAlertsQuery{
		Query: activeAlertsFilter{
			Bool: activeAlertsBool{Must: mustClauses},
		},
		Size: maxActiveAlerts,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: marshal query active alerts: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: build QueryActive request: %w", err)
	}

	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := ix.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: QueryActive: %w", err)
	}

	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return nil, fmt.Errorf("elasticsearch: read QueryActive response: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("elasticsearch: QueryActive returned %d: %s", resp.StatusCode, string(respBody))
	}

	return parseSearchHits(respBody)
}

// searchResponse is used to decode ES _search hits.
type searchResponse struct {
	Hits struct {
		Hits []struct {
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// parseSearchHits decodes a raw ES _search response into domain.Alert slices.
func parseSearchHits(data []byte) ([]domain.Alert, error) {
	var sr searchResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("elasticsearch: decode search response: %w", err)
	}

	alerts := make([]domain.Alert, 0, len(sr.Hits.Hits))

	for i, hit := range sr.Hits.Hits {
		var a domain.Alert
		if unmarshalErr := json.Unmarshal(hit.Source, &a); unmarshalErr != nil {
			return nil, fmt.Errorf("elasticsearch: decode hit %d: %w", i, unmarshalErr)
		}

		alerts = append(alerts, a)
	}

	return alerts, nil
}

// bulkActionMeta is the action line for a _bulk index request.
type bulkActionMeta struct {
	Index bulkIndexTarget `json:"index"`
}

type bulkIndexTarget struct {
	Index string `json:"_index"`
	ID    string `json:"_id"`
}

// BulkIndex writes many alerts in a single _bulk request.
// Used by the backfill subcommand (WP17); not called from the hot polling path.
func (ix *Indexer) BulkIndex(ctx context.Context, alerts []domain.Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	var buf bytes.Buffer

	for i := range alerts {
		meta := bulkActionMeta{
			Index: bulkIndexTarget{Index: ix.index, ID: alerts[i].ID},
		}

		metaLine, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("elasticsearch: marshal bulk meta for alert %s: %w", alerts[i].ID, err)
		}

		docLine, docErr := json.Marshal(alerts[i])
		if docErr != nil {
			return fmt.Errorf("elasticsearch: marshal bulk doc for alert %s: %w", alerts[i].ID, docErr)
		}

		buf.Write(metaLine)
		buf.WriteByte('\n')
		buf.Write(docLine)
		buf.WriteByte('\n')
	}

	bulkURL := fmt.Sprintf("%s/_bulk", ix.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bulkURL, &buf)
	if err != nil {
		return fmt.Errorf("elasticsearch: build BulkIndex request: %w", err)
	}

	req.Header.Set("Content-Type", contentTypeNDJSON)

	resp, err := ix.client.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch: BulkIndex: %w", err)
	}

	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return fmt.Errorf("elasticsearch: read BulkIndex response: %w", readErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("elasticsearch: BulkIndex returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

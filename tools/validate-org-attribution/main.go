// Command validate-org-attribution measures the fraction of lead-pipeline
// documents that carry a populated organization_name_normalized field.
//
// Producers (signal-crawler adapters, classifier need-signal extractor,
// rfp-ingestor parsers) all route attribution through signal.Resolve in
// infrastructure/signal. This tool is the regression gate for that wiring
// (toward #639, lead-pipeline spec §Organization attribution).
//
// Exit codes:
//
//	0 — populated rate ≥ threshold
//	1 — populated rate below threshold OR ES error
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultThreshold          = 0.80
	defaultESURL              = "http://localhost:9200"
	defaultHTTPTimeoutSeconds = 15
	rangeField                = "crawled_at"
	// Need-signal documents live under the same wildcard pattern as everything
	// the classifier writes; the exists filter on `need_signal` narrows to
	// lead-pipeline documents only.
	needIndexPattern = "*_classified_content"
	// RFPs bypass the classifier and land in a dedicated index.
	rfpIndexName = "rfp_classified_content"
)

type countResponse struct {
	Count int64 `json:"count"`
}

func main() {
	// Standalone CLI tool: read ELASTICSEARCH_URL directly rather than pulling
	// in infrastructure/config for a single env var.
	esDefault := defaultESURL
	if v := os.Getenv("ELASTICSEARCH_URL"); v != "" { //nolint:forbidigo // see comment above
		esDefault = v
	}
	esURL := flag.String("es", esDefault, "Elasticsearch base URL")
	threshold := flag.Float64("threshold", defaultThreshold, "Minimum populated rate (0.0–1.0)")
	sinceRaw := flag.String("since", "", "Only count documents with crawled_at >= this RFC3339 timestamp or duration (for example 24h)")
	flag.Parse()

	client := &http.Client{Timeout: defaultHTTPTimeoutSeconds * time.Second}
	ctx := context.Background()

	since, err := parseSince(*sinceRaw, time.Now)
	if err != nil {
		failf("invalid -since: %v", err)
	}

	needTotal, err := count(ctx, client, *esURL, needIndexPattern, countQuery("need_signal", since))
	if err != nil {
		failf("need-signal total: %v", err)
	}
	needPopulated, err := count(ctx, client, *esURL, needIndexPattern,
		countQuery("need_signal.organization_name_normalized", since))
	if err != nil {
		failf("need-signal populated: %v", err)
	}

	rfpTotal, err := count(ctx, client, *esURL, rfpIndexName, countQuery("rfp", since))
	if err != nil {
		failf("rfp total: %v", err)
	}
	rfpPopulated, err := count(ctx, client, *esURL, rfpIndexName,
		countQuery("rfp.organization_name_normalized", since))
	if err != nil {
		failf("rfp populated: %v", err)
	}

	totalDocs := needTotal + rfpTotal
	totalPopulated := needPopulated + rfpPopulated
	combined := rate(totalPopulated, totalDocs)

	report("need_signal", needPopulated, needTotal)
	report("rfp        ", rfpPopulated, rfpTotal)
	report("combined   ", totalPopulated, totalDocs)

	if totalDocs == 0 {
		fmt.Fprintln(os.Stderr, "no documents found — cannot validate")
		os.Exit(1)
	}
	if combined < *threshold {
		fmt.Fprintf(os.Stderr, "FAIL: populated_rate %.4f below threshold %.4f\n", combined, *threshold)
		os.Exit(1)
	}
	fmt.Printf("PASS: populated_rate %.4f ≥ threshold %.4f\n", combined, *threshold)
}

func parseSince(raw string, now func() time.Time) (string, error) {
	if raw == "" {
		return "", nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.Format(time.RFC3339Nano), nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return "", fmt.Errorf("expected RFC3339 timestamp or duration: %w", err)
	}
	if duration <= 0 {
		return "", errors.New("duration must be positive")
	}
	return now().UTC().Add(-duration).Format(time.RFC3339Nano), nil
}

func countQuery(existsField, since string) map[string]any {
	filters := []map[string]any{
		{"exists": map[string]any{"field": existsField}},
	}
	if since != "" {
		filters = append(filters, map[string]any{
			"range": map[string]any{
				rangeField: map[string]any{"gte": since},
			},
		})
	}
	return map[string]any{"query": map[string]any{"bool": map[string]any{"filter": filters}}}
}

func count(ctx context.Context, client *http.Client, esURL, index string, body map[string]any) (int64, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal body: %w", err)
	}
	url := esURL + "/" + index + "/_count"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, fmt.Errorf("read response: %w", readErr)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed countResponse
	if unmarshalErr := json.Unmarshal(respBody, &parsed); unmarshalErr != nil {
		return 0, fmt.Errorf("decode response: %w", unmarshalErr)
	}
	return parsed.Count, nil
}

func rate(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func report(label string, populated, total int64) {
	fmt.Printf("%s: %d / %d populated (%.4f)\n", label, populated, total, rate(populated, total))
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

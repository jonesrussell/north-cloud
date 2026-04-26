// Command validate-sector-alignment measures classifier sector_alignment health.
//
// It emits one structured JSON report with two gates:
//   - coverage: fraction of recently classified docs with non-empty icp.segments[]
//   - accuracy: per-segment precision/recall/F1 against classifier/testdata/icp_labels.yml
//
// Exit codes:
//   - 0: both gates pass
//   - 1: a gate fails, input is invalid, or Elasticsearch is unavailable
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
	"slices"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/icp"
	"gopkg.in/yaml.v3"
)

const (
	defaultAccuracyThreshold  = 0.25
	defaultCoverageThreshold  = 0.25
	defaultESURL              = "http://localhost:9200"
	defaultHTTPTimeoutSeconds = 15
	defaultSince              = "24h"
	indexPattern              = "*_classified_content"
	rangeField                = "crawled_at"
)

var requiredSegments = []string{
	"indigenous_channel",
	"northern_ontario_industry",
	"private_sector_smb",
}

type labelsFile struct {
	SegmentSchemaVersion int      `yaml:"segment_schema_version"`
	Segments             []string `yaml:"segments"`
	Labels               []label  `yaml:"labels"`
}

type label struct {
	DocID    string            `yaml:"doc_id"`
	Title    string            `yaml:"title"`
	Excerpt  string            `yaml:"excerpt"`
	Segments map[string]string `yaml:"segments"`
}

type countResponse struct {
	Count int64 `json:"count"`
}

type report struct {
	GeneratedAt string         `json:"generated_at"`
	Pass        bool           `json:"pass"`
	Coverage    coverageReport `json:"coverage"`
	Accuracy    accuracyReport `json:"accuracy"`
	Notes       []string       `json:"notes,omitempty"`
}

type coverageReport struct {
	IndexPattern      string           `json:"index_pattern"`
	Since             string           `json:"since"`
	Threshold         float64          `json:"threshold"`
	TotalDocs         int64            `json:"total_docs"`
	DocsWithSegments  int64            `json:"docs_with_segments"`
	PopulatedRate     float64          `json:"populated_rate"`
	SegmentHitCounts  map[string]int64 `json:"segment_hit_counts"`
	SMBSignalObserved bool             `json:"smb_signal_observed"`
	Pass              bool             `json:"pass"`
}

type accuracyReport struct {
	SeedModelVersion string                    `json:"seed_model_version"`
	LabelsPath       string                    `json:"labels_path"`
	Threshold        float64                   `json:"threshold"`
	Segments         map[string]segmentMetrics `json:"segments"`
	Pass             bool                      `json:"pass"`
}

type segmentMetrics struct {
	TruePositive   int64   `json:"true_positive"`
	FalsePositive  int64   `json:"false_positive"`
	FalseNegative  int64   `json:"false_negative"`
	TrueNegative   int64   `json:"true_negative"`
	IgnoredPartial int64   `json:"ignored_partial"`
	Precision      float64 `json:"precision"`
	Recall         float64 `json:"recall"`
	F1             float64 `json:"f1"`
	Pass           bool    `json:"pass"`
}

func main() {
	esDefault := defaultESURL
	if v := os.Getenv("ELASTICSEARCH_URL"); v != "" { //nolint:forbidigo // standalone CLI reads one env var directly
		esDefault = v
	}
	esURL := flag.String("es", esDefault, "Elasticsearch base URL")
	sinceRaw := flag.String("since", defaultSince, "Only count documents with crawled_at >= this RFC3339 timestamp or duration")
	coverageThreshold := flag.Float64("coverage-threshold", defaultCoverageThreshold, "Minimum sector_alignment coverage rate")
	accuracyThreshold := flag.Float64("accuracy-threshold", defaultAccuracyThreshold, "Minimum per-segment held-out F1")
	seedPath := flag.String("seed", "source-manager/data/icp-segments.yml", "Path to ICP seed YAML")
	labelsPath := flag.String("labels", "classifier/testdata/icp_labels.yml", "Path to ICP labels YAML")
	flag.Parse()

	if err := validateThreshold(*coverageThreshold); err != nil {
		fail("invalid -coverage-threshold: %v", err)
	}
	if err := validateThreshold(*accuracyThreshold); err != nil {
		fail("invalid -accuracy-threshold: %v", err)
	}

	since, err := parseSince(*sinceRaw, time.Now)
	if err != nil {
		fail("invalid -since: %v", err)
	}

	seed, err := icp.LoadSeed(*seedPath)
	if err != nil {
		fail("seed validation failed: %v", err)
	}
	labels, err := loadLabels(*labelsPath)
	if err != nil {
		fail("labels validation failed: %v", err)
	}

	ctx := context.Background()
	client := &http.Client{Timeout: defaultHTTPTimeoutSeconds * time.Second}
	coverage, err := measureCoverage(ctx, client, strings.TrimRight(*esURL, "/"), since, *coverageThreshold)
	if err != nil {
		fail("coverage validation failed: %v", err)
	}
	accuracy := measureAccuracy(seed, labels, *labelsPath, *accuracyThreshold)

	out := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Coverage:    coverage,
		Accuracy:    accuracy,
		Pass:        coverage.Pass && accuracy.Pass,
	}
	if !coverage.SMBSignalObserved {
		out.Notes = append(out.Notes,
			"private_sector_smb had zero production segment hits in this window; treat that as corpus/ingestion coverage risk before reading it as classifier accuracy regression")
	}

	encoded, marshalErr := json.MarshalIndent(out, "", "  ")
	if marshalErr != nil {
		fail("marshal report: %v", marshalErr)
	}
	fmt.Println(string(encoded))
	if !out.Pass {
		os.Exit(1)
	}
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

func validateThreshold(value float64) error {
	if value < 0 || value > 1 {
		return errors.New("must be between 0.0 and 1.0")
	}
	return nil
}

func loadLabels(path string) ([]label, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file labelsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.SegmentSchemaVersion != 1 {
		return nil, fmt.Errorf("segment_schema_version must be 1, got %d", file.SegmentSchemaVersion)
	}
	for _, segment := range requiredSegments {
		if !slices.Contains(file.Segments, segment) {
			return nil, fmt.Errorf("missing labels segment %q", segment)
		}
	}
	if len(file.Labels) == 0 {
		return nil, errors.New("labels must contain at least one item")
	}
	for i, item := range file.Labels {
		if strings.TrimSpace(item.DocID) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Excerpt) == "" {
			return nil, fmt.Errorf("labels[%d] doc_id, title, and excerpt are required", i)
		}
		for _, segment := range requiredSegments {
			value, ok := item.Segments[segment]
			if !ok {
				return nil, fmt.Errorf("labels[%d].segments missing %q", i, segment)
			}
			if !slices.Contains([]string{"strong", "partial", "none"}, value) {
				return nil, fmt.Errorf("labels[%d].segments.%s is invalid", i, segment)
			}
		}
	}
	return file.Labels, nil
}

func measureCoverage(ctx context.Context, client *http.Client, esURL, since string, threshold float64) (coverageReport, error) {
	total, err := count(ctx, client, esURL, indexPattern, coverageTotalQuery(since))
	if err != nil {
		return coverageReport{}, fmt.Errorf("total docs: %w", err)
	}
	withSegments, err := count(ctx, client, esURL, indexPattern, coverageAnySegmentQuery(since))
	if err != nil {
		return coverageReport{}, fmt.Errorf("docs with ICP segments: %w", err)
	}
	segmentHits := make(map[string]int64, len(requiredSegments))
	for _, segment := range requiredSegments {
		hits, hitErr := count(ctx, client, esURL, indexPattern, coverageSegmentQuery(segment, since))
		if hitErr != nil {
			return coverageReport{}, fmt.Errorf("segment %s hits: %w", segment, hitErr)
		}
		segmentHits[segment] = hits
	}
	populatedRate := rate(withSegments, total)
	return coverageReport{
		IndexPattern:      indexPattern,
		Since:             since,
		Threshold:         threshold,
		TotalDocs:         total,
		DocsWithSegments:  withSegments,
		PopulatedRate:     populatedRate,
		SegmentHitCounts:  segmentHits,
		SMBSignalObserved: segmentHits["private_sector_smb"] > 0,
		Pass:              total > 0 && populatedRate >= threshold,
	}, nil
}

func coverageTotalQuery(since string) map[string]any {
	return map[string]any{"query": map[string]any{"bool": map[string]any{"filter": rangeFilters(since)}}}
}

func coverageAnySegmentQuery(since string) map[string]any {
	filters := rangeFilters(since)
	filters = append(filters, map[string]any{
		"nested": map[string]any{
			"path":  "icp.segments",
			"query": map[string]any{"exists": map[string]any{"field": "icp.segments.segment"}},
		},
	})
	return map[string]any{"query": map[string]any{"bool": map[string]any{"filter": filters}}}
}

func coverageSegmentQuery(segment, since string) map[string]any {
	filters := rangeFilters(since)
	filters = append(filters, map[string]any{
		"nested": map[string]any{
			"path": "icp.segments",
			"query": map[string]any{
				"term": map[string]any{"icp.segments.segment": segment},
			},
		},
	})
	return map[string]any{"query": map[string]any{"bool": map[string]any{"filter": filters}}}
}

func rangeFilters(since string) []map[string]any {
	if since == "" {
		return []map[string]any{}
	}
	return []map[string]any{{
		"range": map[string]any{
			rangeField: map[string]any{"gte": since},
		},
	}}
}

func count(ctx context.Context, client *http.Client, esURL, index string, body map[string]any) (int64, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, esURL+"/"+index+"/_count", bytes.NewReader(buf))
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

func measureAccuracy(seed *icp.Seed, labels []label, labelsPath string, threshold float64) accuracyReport {
	metrics := make(map[string]segmentMetrics, len(requiredSegments))
	for _, segment := range requiredSegments {
		metrics[segment] = segmentMetrics{}
	}
	for _, item := range labels {
		predicted := predictedSegments(seed, item)
		for _, segment := range requiredSegments {
			m := metrics[segment]
			switch item.Segments[segment] {
			case "partial":
				m.IgnoredPartial++
			case "strong":
				if predicted[segment] {
					m.TruePositive++
				} else {
					m.FalseNegative++
				}
			case "none":
				if predicted[segment] {
					m.FalsePositive++
				} else {
					m.TrueNegative++
				}
			}
			metrics[segment] = m
		}
	}

	pass := true
	for _, segment := range requiredSegments {
		m := metrics[segment]
		m.Precision = rate(m.TruePositive, m.TruePositive+m.FalsePositive)
		m.Recall = rate(m.TruePositive, m.TruePositive+m.FalseNegative)
		m.F1 = f1(m.Precision, m.Recall)
		if m.TruePositive+m.FalseNegative == 0 {
			m.Pass = m.FalsePositive == 0
		} else {
			m.Pass = m.F1 >= threshold
		}
		if !m.Pass {
			pass = false
		}
		metrics[segment] = m
	}

	return accuracyReport{
		SeedModelVersion: icp.ModelVersionV1,
		LabelsPath:       labelsPath,
		Threshold:        threshold,
		Segments:         metrics,
		Pass:             pass,
	}
}

func predictedSegments(seed *icp.Seed, item label) map[string]bool {
	predicted := make(map[string]bool, len(requiredSegments))
	result := icp.Match(seed, icp.Document{
		Title: item.Title,
		Body:  item.Excerpt,
	})
	if result == nil {
		return predicted
	}
	for _, segment := range result.Segments {
		predicted[segment.Segment] = true
	}
	return predicted
}

func rate(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func f1(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

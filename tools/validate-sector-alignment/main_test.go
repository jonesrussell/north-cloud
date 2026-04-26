package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/icp"
)

func TestParseSinceDuration(t *testing.T) {
	t.Parallel()

	now := func() time.Time {
		return time.Date(2026, 4, 26, 20, 0, 0, 0, time.UTC)
	}
	got, err := parseSince("24h", now)
	if err != nil {
		t.Fatalf("parseSince returned error: %v", err)
	}
	if got != "2026-04-25T20:00:00Z" {
		t.Fatalf("parseSince() = %q", got)
	}
}

func TestParseSinceRejectsNegativeDuration(t *testing.T) {
	t.Parallel()

	if _, err := parseSince("-1h", time.Now); err == nil {
		t.Fatal("parseSince accepted negative duration")
	}
}

func TestCoverageSegmentQueryUsesNestedSegmentFilter(t *testing.T) {
	t.Parallel()

	body := coverageSegmentQuery("private_sector_smb", "2026-04-25T20:00:00Z")
	mustJSON(t, body)

	filters := queryFilters(t, body)
	if len(filters) != 2 {
		t.Fatalf("len(filters) = %d", len(filters))
	}
	nested, ok := filters[1]["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested filter missing: %#v", filters[1])
	}
	if got := nested["path"]; got != "icp.segments" {
		t.Fatalf("nested path = %v", got)
	}
	query := nested["query"].(map[string]any)
	term := query["term"].(map[string]any)
	if got := term["icp.segments.segment"]; got != "private_sector_smb" {
		t.Fatalf("segment term = %v", got)
	}
}

func TestMeasureAccuracyIgnoresPartialLabels(t *testing.T) {
	t.Parallel()

	seed := &icp.Seed{
		SegmentSchemaVersion: 1,
		SeedUpdatedAt:        "2026-04-26",
		Segments: []icp.Segment{
			{Name: "indigenous_channel", Description: "Indigenous", Keywords: []string{"First Nation"}, MinScore: 0.30},
			{Name: "northern_ontario_industry", Description: "NOI", Keywords: []string{"Sudbury"}, MinScore: 0.30},
			{Name: "private_sector_smb", Description: "SMB", Keywords: []string{"privately held"}, MinScore: 0.30},
		},
	}
	labels := []label{
		{
			DocID:   "strong_hit",
			Title:   "First Nation broadband authority",
			Excerpt: "A First Nation project in Canada",
			Segments: map[string]string{
				"indigenous_channel":        "strong",
				"northern_ontario_industry": "none",
				"private_sector_smb":        "none",
			},
		},
		{
			DocID:   "partial_ignored",
			Title:   "Sudbury consultancy",
			Excerpt: "A privately held firm",
			Segments: map[string]string{
				"indigenous_channel":        "none",
				"northern_ontario_industry": "partial",
				"private_sector_smb":        "strong",
			},
		},
	}

	report := measureAccuracy(seed, labels, "labels.yml", 0.01)
	if !report.Pass {
		t.Fatal("measureAccuracy() did not pass")
	}
	if got := report.Segments["northern_ontario_industry"].IgnoredPartial; got != 1 {
		t.Fatalf("IgnoredPartial = %d", got)
	}
	if got := report.Segments["private_sector_smb"].TruePositive; got != 1 {
		t.Fatalf("private_sector_smb TP = %d", got)
	}
}

func queryFilters(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()

	query, ok := body["query"].(map[string]any)
	if !ok {
		t.Fatalf("query missing: %#v", body)
	}
	boolQuery, ok := query["bool"].(map[string]any)
	if !ok {
		t.Fatalf("bool query missing: %#v", query)
	}
	filters, ok := boolQuery["filter"].([]map[string]any)
	if !ok {
		t.Fatalf("filter missing: %#v", boolQuery)
	}
	return filters
}

func mustJSON(t *testing.T, value any) {
	t.Helper()

	if _, err := json.Marshal(value); err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
}

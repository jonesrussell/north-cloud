package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseSinceRFC3339(t *testing.T) {
	t.Parallel()

	got, err := parseSince("2026-04-19T14:05:00Z", time.Now)
	if err != nil {
		t.Fatalf("parseSince returned error: %v", err)
	}
	if got != "2026-04-19T14:05:00Z" {
		t.Fatalf("parseSince() = %q", got)
	}
}

func TestParseSinceDuration(t *testing.T) {
	t.Parallel()

	now := func() time.Time {
		return time.Date(2026, 4, 20, 14, 5, 0, 0, time.UTC)
	}
	got, err := parseSince("24h", now)
	if err != nil {
		t.Fatalf("parseSince returned error: %v", err)
	}
	if got != "2026-04-19T14:05:00Z" {
		t.Fatalf("parseSince() = %q", got)
	}
}

func TestParseSinceRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	if _, err := parseSince("yesterday", time.Now); err == nil {
		t.Fatal("parseSince accepted invalid value")
	}
}

func TestCountQueryIncludesSinceFilter(t *testing.T) {
	t.Parallel()

	body := countQuery("need_signal", "2026-04-19T14:05:00Z")
	mustJSON(t, body)

	filters := queryFilters(t, body)
	if len(filters) != 2 {
		t.Fatalf("len(filters) = %d", len(filters))
	}
	existsFilter, ok := filters[0]["exists"].(map[string]any)
	if !ok {
		t.Fatalf("exists filter missing: %#v", filters[0])
	}
	if got := existsFilter["field"]; got != "need_signal" {
		t.Fatalf("exists field = %v", got)
	}
	rangeFilter, ok := filters[1]["range"].(map[string]any)
	if !ok {
		t.Fatalf("range filter missing: %#v", filters[1])
	}
	crawledAt, ok := rangeFilter[rangeField].(map[string]any)
	if !ok {
		t.Fatalf("crawled_at range missing: %#v", rangeFilter)
	}
	if got := crawledAt["gte"]; got != "2026-04-19T14:05:00Z" {
		t.Fatalf("range gte = %v", got)
	}
}

func TestCountQueryOmitsSinceFilterWhenEmpty(t *testing.T) {
	t.Parallel()

	body := countQuery("rfp", "")
	mustJSON(t, body)

	filters := queryFilters(t, body)
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d", len(filters))
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

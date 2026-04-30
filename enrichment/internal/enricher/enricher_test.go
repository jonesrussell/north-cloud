package enricher

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

func TestRegistryOnlyContainsSupportedTypes(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistry(&fakeSearcher{})
	types := registry.Types()
	sort.Strings(types)

	want := []string{TypeCompanyIntel, TypeHiring, TypeTechStack}
	sort.Strings(want)
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("types = %#v, want %#v", types, want)
	}

	if _, ok := registry.Lookup("market_news"); ok {
		t.Fatal("market_news unexpectedly registered")
	}
}

func TestUnknownResultIsSkipped(t *testing.T) {
	t.Parallel()

	result := UnknownResult(validRequest(t), "market_news")

	if result.Status != StatusSkipped {
		t.Fatalf("status = %q, want %q", result.Status, StatusSkipped)
	}
	if result.Type != "market_news" {
		t.Fatalf("type = %q, want market_news", result.Type)
	}
}

func TestCompanyIntelHappyPath(t *testing.T) {
	t.Parallel()

	searcher := &fakeSearcher{hits: evidenceHits(t)}
	result, err := NewCompanyIntel(searcher).Enrich(context.Background(), validRequest(t))

	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	assertResult(t, result, TypeCompanyIntel, StatusSuccess)
	if result.Confidence != highConfidenceThreshold {
		t.Fatalf("confidence = %.2f, want %.2f", result.Confidence, highConfidenceThreshold)
	}
	assertQueryContains(t, searcher.lastRequest.Query, "Acme Mining")
	assertDataHasKey(t, result.Data, "summary")
}

func TestTechStackHappyPathDetectsTermsDeterministically(t *testing.T) {
	t.Parallel()

	searcher := &fakeSearcher{hits: evidenceHits(t)}
	result, err := NewTechStack(searcher).Enrich(context.Background(), validRequest(t))

	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	assertResult(t, result, TypeTechStack, StatusSuccess)

	got, ok := result.Data["technologies"].([]string)
	if !ok {
		t.Fatalf("technologies = %#v, want []string", result.Data["technologies"])
	}
	want := []string{"analytics", "cloud"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("technologies = %#v, want %#v", got, want)
	}
}

func TestHiringHappyPath(t *testing.T) {
	t.Parallel()

	searcher := &fakeSearcher{hits: evidenceHits(t)}
	result, err := NewHiring(searcher).Enrich(context.Background(), validRequest(t))

	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	assertResult(t, result, TypeHiring, StatusSuccess)
	if got := result.Data["hiring_signal_count"]; got != 2 {
		t.Fatalf("hiring signal count = %#v, want 2", got)
	}
}

func TestEnrichersReturnEmptyResultWhenNoEvidence(t *testing.T) {
	t.Parallel()

	for _, item := range []struct {
		name     string
		enricher Enricher
	}{
		{name: TypeCompanyIntel, enricher: NewCompanyIntel(&fakeSearcher{})},
		{name: TypeTechStack, enricher: NewTechStack(&fakeSearcher{})},
		{name: TypeHiring, enricher: NewHiring(&fakeSearcher{})},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()

			result, err := item.enricher.Enrich(context.Background(), validRequest(t))
			if err != nil {
				t.Fatalf("enrich: %v", err)
			}
			assertResult(t, result, item.name, StatusEmpty)
			if result.Confidence != emptyConfidence {
				t.Fatalf("confidence = %.2f, want %.2f", result.Confidence, emptyConfidence)
			}
		})
	}
}

func TestEnrichersSurfaceSearchErrors(t *testing.T) {
	t.Parallel()

	searchErr := errors.New("es unavailable")
	for _, item := range []struct {
		name     string
		enricher Enricher
	}{
		{name: TypeCompanyIntel, enricher: NewCompanyIntel(&fakeSearcher{err: searchErr})},
		{name: TypeTechStack, enricher: NewTechStack(&fakeSearcher{err: searchErr})},
		{name: TypeHiring, enricher: NewHiring(&fakeSearcher{err: searchErr})},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()

			result, err := item.enricher.Enrich(context.Background(), validRequest(t))
			if !errors.Is(err, searchErr) {
				t.Fatalf("error = %v, want %v", err, searchErr)
			}
			assertResult(t, result, item.name, StatusError)
			if !strings.Contains(result.Error, searchErr.Error()) {
				t.Fatalf("result error = %q, want search error", result.Error)
			}
		})
	}
}

type fakeSearcher struct {
	hits        []Hit
	err         error
	lastRequest SearchRequest
}

func (s *fakeSearcher) Search(_ context.Context, request SearchRequest) ([]Hit, error) {
	s.lastRequest = request
	if s.err != nil {
		return nil, s.err
	}
	return s.hits, nil
}

func validRequest(t *testing.T) api.EnrichmentRequest {
	t.Helper()

	return api.EnrichmentRequest{
		LeadID:         "lead-123",
		CompanyName:    "Acme Mining",
		Domain:         "acme.example",
		Sector:         "mining",
		RequestedTypes: []string{TypeCompanyIntel, TypeTechStack, TypeHiring},
		CallbackURL:    "https://waaseyaa.example/callback",
		CallbackAPIKey: "secret",
	}
}

func evidenceHits(t *testing.T) []Hit {
	t.Helper()

	return []Hit{
		{
			ID:    "doc-1",
			Score: 7,
			Source: map[string]any{
				"title":        "Acme Mining expands cloud analytics hiring",
				"body":         "Acme Mining uses cloud analytics and is hiring new operations positions.",
				"domain":       "acme.example",
				"sector":       "mining",
				"url":          "https://acme.example/news",
				"source":       "classified_content",
				"published_at": "2026-04-01",
			},
		},
		{
			ID:    "doc-2",
			Score: 4,
			Source: map[string]any{
				"title":  "Careers update",
				"body":   "New jobs and careers page updates for Acme Mining.",
				"domain": "acme.example",
			},
		},
	}
}

func assertResult(t *testing.T, result Result, enrichmentType string, status string) {
	t.Helper()

	if result.LeadID != "lead-123" {
		t.Fatalf("lead id = %q, want lead-123", result.LeadID)
	}
	if result.Type != enrichmentType {
		t.Fatalf("type = %q, want %q", result.Type, enrichmentType)
	}
	if result.Status != status {
		t.Fatalf("status = %q, want %q", result.Status, status)
	}
}

func assertDataHasKey(t *testing.T, data map[string]any, key string) {
	t.Helper()

	if _, ok := data[key]; !ok {
		t.Fatalf("data missing key %q: %#v", key, data)
	}
}

func assertQueryContains(t *testing.T, query map[string]any, want string) {
	t.Helper()

	if !strings.Contains(toString(query), want) {
		t.Fatalf("query = %#v, want to contain %q", query, want)
	}
}

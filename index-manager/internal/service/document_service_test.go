package service //nolint:testpackage // testing unexported mapping methods

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
)

var errTestES = errors.New("test ES error")

// --- mapToDocument ---

func allFieldsSource() map[string]any {
	return map[string]any{
		"title":          "Test Article",
		"url":            "https://example.com/article",
		"source_name":    "example_com",
		"content_type":   "article",
		"quality_score":  float64(85),
		"body":           "Article body text",
		"raw_text":       "Raw text content",
		"raw_html":       "<p>HTML</p>",
		"topics":         []any{"crime", "local"},
		"published_date": "2024-01-15T10:00:00Z",
		"crawled_at":     "2024-01-15T12:00:00Z",
		"crime": map[string]any{
			"sub_label":          "robbery",
			"primary_crime_type": "theft",
			"relevance":          "core_street_crime",
			"crime_types":        []any{"robbery", "theft"},
			"final_confidence":   float64(0.95),
			"homepage_eligible":  true,
			"review_required":    false,
			"model_version":      "v2.1",
		},
		"location": map[string]any{
			"city":        "Sudbury",
			"province":    "Ontario",
			"country":     "Canada",
			"specificity": "city",
			"confidence":  float64(0.88),
		},
		"extra_field": "extra_value",
	}
}

func assertBaseFields(t *testing.T, doc *domain.Document) {
	t.Helper()

	if doc.ID != "doc-123" {
		t.Errorf("ID = %q, want %q", doc.ID, "doc-123")
	}
	if doc.Title != "Test Article" {
		t.Errorf("Title = %q, want %q", doc.Title, "Test Article")
	}
	if doc.URL != "https://example.com/article" {
		t.Errorf("URL = %q, want %q", doc.URL, "https://example.com/article")
	}
	if doc.SourceName != "example_com" {
		t.Errorf("SourceName = %q, want %q", doc.SourceName, "example_com")
	}
	if doc.ContentType != "article" {
		t.Errorf("ContentType = %q, want %q", doc.ContentType, "article")
	}
	if doc.QualityScore != 85 {
		t.Errorf("QualityScore = %d, want 85", doc.QualityScore)
	}
	if doc.Body != "Article body text" {
		t.Errorf("Body = %q, want %q", doc.Body, "Article body text")
	}
	if doc.RawText != "Raw text content" {
		t.Errorf("RawText = %q, want %q", doc.RawText, "Raw text content")
	}
	if doc.RawHTML != "<p>HTML</p>" {
		t.Errorf("RawHTML = %q, want %q", doc.RawHTML, "<p>HTML</p>")
	}
	if len(doc.Topics) != 2 {
		t.Fatalf("Topics length = %d, want 2", len(doc.Topics))
	}
	if doc.Topics[0] != "crime" || doc.Topics[1] != "local" {
		t.Errorf("Topics = %v, want [crime local]", doc.Topics)
	}
	if doc.PublishedDate == nil {
		t.Fatal("PublishedDate should not be nil")
	}
	if doc.CrawledAt == nil {
		t.Fatal("CrawledAt should not be nil")
	}
}

func assertCrimeFields(t *testing.T, doc *domain.Document) {
	t.Helper()

	if doc.Crime == nil {
		t.Fatal("Crime should not be nil")
	}
	if doc.Crime.SubLabel != "robbery" {
		t.Errorf("Crime.SubLabel = %q, want %q", doc.Crime.SubLabel, "robbery")
	}
	if doc.Crime.PrimaryCrimeType != "theft" {
		t.Errorf("Crime.PrimaryCrimeType = %q, want %q", doc.Crime.PrimaryCrimeType, "theft")
	}
	if doc.Crime.Relevance != "core_street_crime" {
		t.Errorf("Crime.Relevance = %q, want %q", doc.Crime.Relevance, "core_street_crime")
	}
	if len(doc.Crime.CrimeTypes) != 2 {
		t.Fatalf("Crime.CrimeTypes length = %d, want 2", len(doc.Crime.CrimeTypes))
	}
	if doc.Crime.Confidence != 0.95 {
		t.Errorf("Crime.Confidence = %f, want 0.95", doc.Crime.Confidence)
	}
	if !doc.Crime.HomepageEligible {
		t.Error("Crime.HomepageEligible should be true")
	}
	if doc.Crime.ReviewRequired {
		t.Error("Crime.ReviewRequired should be false")
	}
	if doc.Crime.ModelVersion != "v2.1" {
		t.Errorf("Crime.ModelVersion = %q, want %q", doc.Crime.ModelVersion, "v2.1")
	}
}

func assertLocationFields(t *testing.T, doc *domain.Document) {
	t.Helper()

	if doc.Location == nil {
		t.Fatal("Location should not be nil")
	}
	if doc.Location.City != "Sudbury" {
		t.Errorf("Location.City = %q, want %q", doc.Location.City, "Sudbury")
	}
	if doc.Location.Province != "Ontario" {
		t.Errorf("Location.Province = %q, want %q", doc.Location.Province, "Ontario")
	}
	if doc.Location.Country != "Canada" {
		t.Errorf("Location.Country = %q, want %q", doc.Location.Country, "Canada")
	}
	if doc.Location.Specificity != "city" {
		t.Errorf("Location.Specificity = %q, want %q", doc.Location.Specificity, "city")
	}
	if doc.Location.Confidence != 0.88 {
		t.Errorf("Location.Confidence = %f, want 0.88", doc.Location.Confidence)
	}
}

func TestMapToDocument_AllFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	doc := svc.mapToDocument("doc-123", allFieldsSource())

	t.Run("base_fields", func(t *testing.T) {
		assertBaseFields(t, doc)
	})

	t.Run("crime_fields", func(t *testing.T) {
		assertCrimeFields(t, doc)
	})

	t.Run("location_fields", func(t *testing.T) {
		assertLocationFields(t, doc)
	})

	t.Run("meta_fields", func(t *testing.T) {
		if doc.Meta["extra_field"] != "extra_value" {
			t.Errorf("Meta[extra_field] = %v, want %q", doc.Meta["extra_field"], "extra_value")
		}
	})
}

func TestMapToDocument_MinimalFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{}

	doc := svc.mapToDocument("doc-456", source)

	if doc.ID != "doc-456" {
		t.Errorf("ID = %q, want %q", doc.ID, "doc-456")
	}
	if doc.Title != "" {
		t.Errorf("Title = %q, want empty", doc.Title)
	}
	if doc.Crime != nil {
		t.Error("Crime should be nil for empty source")
	}
	if doc.Location != nil {
		t.Error("Location should be nil for empty source")
	}
	if len(doc.Meta) != 0 {
		t.Errorf("Meta should be empty, got %d entries", len(doc.Meta))
	}
}

func TestMapToDocument_LegacyCrimeBoolean(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"is_crime_related": true,
	}

	doc := svc.mapToDocument("doc-789", source)

	if doc.Crime == nil {
		t.Fatal("Crime should not be nil for legacy is_crime_related=true")
	}
	if doc.Crime.Relevance != "core_street_crime" {
		t.Errorf("Crime.Relevance = %q, want %q", doc.Crime.Relevance, "core_street_crime")
	}
}

func TestMapToDocument_LegacyCrimeFalse(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"is_crime_related": false,
	}

	doc := svc.mapToDocument("doc-000", source)

	if doc.Crime != nil {
		t.Error("Crime should be nil for is_crime_related=false")
	}
}

func TestMapToDocument_InvalidDateIgnored(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"published_date": "not-a-date",
		"crawled_at":     "also-not-a-date",
	}

	doc := svc.mapToDocument("doc-inv", source)

	if doc.PublishedDate != nil {
		t.Error("PublishedDate should be nil for invalid date string")
	}
	if doc.CrawledAt != nil {
		t.Error("CrawledAt should be nil for invalid date string")
	}
}

func TestMapToDocument_TopicsWithNonStrings(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"topics": []any{"valid_topic", float64(123), nil},
	}

	doc := svc.mapToDocument("doc-topics", source)

	if len(doc.Topics) != 1 {
		t.Errorf("Topics length = %d, want 1 (non-strings skipped)", len(doc.Topics))
	}
	if doc.Topics[0] != "valid_topic" {
		t.Errorf("Topics[0] = %q, want %q", doc.Topics[0], "valid_topic")
	}
}

// --- documentToMap ---

func TestDocumentToMap_AllFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	now := time.Now()

	doc := &domain.Document{
		Title:         "Test",
		URL:           "https://example.com",
		SourceName:    "example_com",
		ContentType:   "article",
		QualityScore:  80,
		Body:          "body text",
		RawText:       "raw text",
		RawHTML:       "<p>html</p>",
		Topics:        []string{"crime"},
		PublishedDate: &now,
		CrawledAt:     &now,
		Crime: &domain.CrimeInfo{
			Relevance:        "core_street_crime",
			SubLabel:         "robbery",
			PrimaryCrimeType: "theft",
			CrimeTypes:       []string{"robbery"},
			Confidence:       0.9,
			HomepageEligible: true,
			ReviewRequired:   false,
			ModelVersion:     "v1",
		},
		Location: &domain.LocationInfo{
			City:        "Sudbury",
			Province:    "Ontario",
			Country:     "Canada",
			Specificity: "city",
			Confidence:  0.85,
		},
		Meta: map[string]any{
			"custom_key": "custom_value",
		},
	}

	result := svc.documentToMap(doc)

	if result["title"] != "Test" {
		t.Errorf("title = %v, want %q", result["title"], "Test")
	}
	if result["url"] != "https://example.com" {
		t.Errorf("url = %v, want %q", result["url"], "https://example.com")
	}
	if result["source_name"] != "example_com" {
		t.Errorf("source_name = %v, want %q", result["source_name"], "example_com")
	}
	if result["content_type"] != "article" {
		t.Errorf("content_type = %v, want %q", result["content_type"], "article")
	}
	if result["quality_score"] != 80 {
		t.Errorf("quality_score = %v, want 80", result["quality_score"])
	}
	if result["custom_key"] != "custom_value" {
		t.Errorf("custom_key = %v, want %q", result["custom_key"], "custom_value")
	}

	// Crime map
	crimeMap, ok := result["crime"].(map[string]any)
	if !ok {
		t.Fatal("crime should be a map")
	}
	if crimeMap["relevance"] != "core_street_crime" {
		t.Errorf("crime.relevance = %v, want %q", crimeMap["relevance"], "core_street_crime")
	}
	if crimeMap["sub_label"] != "robbery" {
		t.Errorf("crime.sub_label = %v, want %q", crimeMap["sub_label"], "robbery")
	}

	// Location map
	locMap, ok := result["location"].(map[string]any)
	if !ok {
		t.Fatal("location should be a map")
	}
	if locMap["city"] != "Sudbury" {
		t.Errorf("location.city = %v, want %q", locMap["city"], "Sudbury")
	}
}

func TestDocumentToMap_EmptyDocument(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	doc := &domain.Document{}

	result := svc.documentToMap(doc)

	// Empty fields should not be included
	if _, exists := result["title"]; exists {
		t.Error("empty title should not be in map")
	}
	if _, exists := result["crime"]; exists {
		t.Error("nil crime should not be in map")
	}
	if _, exists := result["location"]; exists {
		t.Error("nil location should not be in map")
	}
}

// --- crimeInfoToMap ---

func TestCrimeInfoToMap_AllFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	crime := &domain.CrimeInfo{
		SubLabel:         "assault",
		PrimaryCrimeType: "violent",
		Relevance:        "core_street_crime",
		CrimeTypes:       []string{"assault", "battery"},
		Confidence:       0.92,
		HomepageEligible: true,
		ReviewRequired:   true,
		ModelVersion:     "v3",
	}

	result := svc.crimeInfoToMap(crime)

	if result["sub_label"] != "assault" {
		t.Errorf("sub_label = %v, want %q", result["sub_label"], "assault")
	}
	if result["primary_crime_type"] != "violent" {
		t.Errorf("primary_crime_type = %v, want %q", result["primary_crime_type"], "violent")
	}
	if result["relevance"] != "core_street_crime" {
		t.Errorf("relevance = %v, want %q", result["relevance"], "core_street_crime")
	}
	crimeTypes, ok := result["crime_types"].([]string)
	if !ok {
		t.Fatal("crime_types should be []string")
	}
	if len(crimeTypes) != 2 {
		t.Errorf("crime_types length = %d, want 2", len(crimeTypes))
	}
	if result["homepage_eligible"] != true {
		t.Error("homepage_eligible should be true")
	}
	if result["review_required"] != true {
		t.Error("review_required should be true")
	}
	if result["model_version"] != "v3" {
		t.Errorf("model_version = %v, want %q", result["model_version"], "v3")
	}
}

func TestCrimeInfoToMap_MinimalFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	crime := &domain.CrimeInfo{}

	result := svc.crimeInfoToMap(crime)

	// homepage_eligible and review_required are always set (even as false)
	if result["homepage_eligible"] != false {
		t.Error("homepage_eligible should be false")
	}
	if result["review_required"] != false {
		t.Error("review_required should be false")
	}
	// Empty strings should not be set
	if _, exists := result["sub_label"]; exists {
		t.Error("empty sub_label should not be in map")
	}
	if _, exists := result["relevance"]; exists {
		t.Error("empty relevance should not be in map")
	}
}

// --- locationInfoToMap ---

func TestLocationInfoToMap_AllFields(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	loc := &domain.LocationInfo{
		City:        "Toronto",
		Province:    "Ontario",
		Country:     "Canada",
		Specificity: "city",
		Confidence:  0.95,
	}

	result := svc.locationInfoToMap(loc)

	if result["city"] != "Toronto" {
		t.Errorf("city = %v, want %q", result["city"], "Toronto")
	}
	if result["province"] != "Ontario" {
		t.Errorf("province = %v, want %q", result["province"], "Ontario")
	}
	if result["country"] != "Canada" {
		t.Errorf("country = %v, want %q", result["country"], "Canada")
	}
	if result["specificity"] != "city" {
		t.Errorf("specificity = %v, want %q", result["specificity"], "city")
	}
	if result["confidence"] != 0.95 {
		t.Errorf("confidence = %v, want 0.95", result["confidence"])
	}
}

func TestLocationInfoToMap_EmptyLocation(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	loc := &domain.LocationInfo{}

	result := svc.locationInfoToMap(loc)

	if len(result) != 0 {
		t.Errorf("empty location should produce empty map, got %d entries", len(result))
	}
}

// --- NewDocumentService ---

func TestNewDocumentService(t *testing.T) {
	t.Helper()

	svc := NewDocumentService(nil, &noopLogger{})
	if svc == nil {
		t.Fatal("NewDocumentService() returned nil")
	}
	if svc.queryBuilder == nil {
		t.Error("queryBuilder should be initialized")
	}
}

// --- extractCrimeInfo edge cases ---

func TestExtractCrimeInfo_NoCrimeData(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"title": "No crime here",
	}

	crime := svc.extractCrimeInfo(source)
	if crime != nil {
		t.Error("expected nil crime for source without crime data")
	}
}

func TestExtractCrimeInfo_EmptyCrimeTypes(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"crime": map[string]any{
			"relevance": "not_crime",
		},
	}

	crime := svc.extractCrimeInfo(source)
	if crime == nil {
		t.Fatal("expected non-nil crime info")
	}
	if crime.Relevance != "not_crime" {
		t.Errorf("Relevance = %q, want %q", crime.Relevance, "not_crime")
	}
	if len(crime.CrimeTypes) != 0 {
		t.Errorf("CrimeTypes should be empty, got %d", len(crime.CrimeTypes))
	}
}

// --- extractLocationInfo edge cases ---

func TestExtractLocationInfo_NoLocationData(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{}

	loc := svc.extractLocationInfo(source)
	if loc != nil {
		t.Error("expected nil location for source without location data")
	}
}

func TestExtractLocationInfo_PartialData(t *testing.T) {
	t.Helper()

	svc := &DocumentService{logger: &noopLogger{}}
	source := map[string]any{
		"location": map[string]any{
			"country": "Canada",
		},
	}

	loc := svc.extractLocationInfo(source)
	if loc == nil {
		t.Fatal("expected non-nil location")
	}
	if loc.Country != "Canada" {
		t.Errorf("Country = %q, want %q", loc.Country, "Canada")
	}
	if loc.City != "" {
		t.Errorf("City = %q, want empty", loc.City)
	}
}

// --- Aggregation helper function tests ---

func TestExtractBuckets_ValidData(t *testing.T) {
	t.Helper()

	raw := []byte(`{"buckets":[{"key":"crime","doc_count":10},{"key":"news","doc_count":5}]}`)
	result := extractBuckets(raw)

	if len(result) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(result))
	}
	if result["crime"] != 10 {
		t.Errorf("crime count = %d, want 10", result["crime"])
	}
	if result["news"] != 5 {
		t.Errorf("news count = %d, want 5", result["news"])
	}
}

func TestExtractBuckets_EmptyBuckets(t *testing.T) {
	t.Helper()

	raw := []byte(`{"buckets":[]}`)
	result := extractBuckets(raw)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}

func TestExtractBuckets_InvalidJSON(t *testing.T) {
	t.Helper()

	raw := []byte(`{invalid}`)
	result := extractBuckets(raw)

	if len(result) != 0 {
		t.Errorf("expected empty result for invalid JSON, got %d entries", len(result))
	}
}

func TestExtractBucketKeys_ValidData(t *testing.T) {
	t.Helper()

	raw := []byte(`{"buckets":[{"key":"alpha","doc_count":3},{"key":"beta","doc_count":1}]}`)
	keys := extractBucketKeys(raw)

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "alpha" {
		t.Errorf("keys[0] = %q, want %q", keys[0], "alpha")
	}
	if keys[1] != "beta" {
		t.Errorf("keys[1] = %q, want %q", keys[1], "beta")
	}
}

func TestExtractBucketKeys_InvalidJSON(t *testing.T) {
	t.Helper()

	raw := []byte(`invalid`)
	keys := extractBucketKeys(raw)

	if keys != nil {
		t.Errorf("expected nil for invalid JSON, got %v", keys)
	}
}

func TestExtractFilterCount_ValidData(t *testing.T) {
	t.Helper()

	raw := []byte(`{"doc_count":42}`)
	count := extractFilterCount(raw)

	if count != 42 {
		t.Errorf("extractFilterCount() = %d, want 42", count)
	}
}

func TestExtractFilterCount_InvalidJSON(t *testing.T) {
	t.Helper()

	raw := []byte(`invalid`)
	count := extractFilterCount(raw)

	if count != 0 {
		t.Errorf("extractFilterCount() = %d, want 0 for invalid JSON", count)
	}
}

func TestExtractContentTypeXCrime_ValidData(t *testing.T) {
	t.Helper()

	raw := []byte(`{
		"buckets": [
			{
				"key": "article",
				"doc_count": 100,
				"crime": {"buckets": [{"key": "core_street_crime", "doc_count": 20}]}
			}
		]
	}`)
	result := extractContentTypeXCrime(raw)

	if len(result) != 1 {
		t.Fatalf("expected 1 content type, got %d", len(result))
	}
	if result["article"]["core_street_crime"] != 20 {
		t.Errorf("article/core_street_crime = %d, want 20", result["article"]["core_street_crime"])
	}
}

func TestExtractContentTypeXCrime_EmptyCrime(t *testing.T) {
	t.Helper()

	raw := []byte(`{
		"buckets": [
			{
				"key": "page",
				"doc_count": 50,
				"crime": {"buckets": []}
			}
		]
	}`)
	result := extractContentTypeXCrime(raw)

	// Empty inner buckets means no entry for that content type
	if _, exists := result["page"]; exists {
		t.Error("expected no entry for page with empty crime buckets")
	}
}

func TestExtractContentTypeXCrime_InvalidJSON(t *testing.T) {
	t.Helper()

	raw := []byte(`invalid`)
	result := extractContentTypeXCrime(raw)

	if len(result) != 0 {
		t.Errorf("expected empty result for invalid JSON, got %d entries", len(result))
	}
}

// --- buildAggregationQuery ---

func TestBuildAggregationQuery_NoFilters(t *testing.T) {
	t.Helper()

	svc := newTestService(&mockESClient{})
	aggs := map[string]any{
		"test_agg": map[string]any{"terms": map[string]any{"field": "test"}},
	}

	query := svc.buildAggregationQuery(nil, aggs)

	if query["size"] != 0 {
		t.Errorf("size = %v, want 0", query["size"])
	}
	if query["track_total_hits"] != true {
		t.Error("track_total_hits should be true")
	}
	if _, hasQuery := query["query"]; hasQuery {
		t.Error("query should not be set when request is nil")
	}
}

func TestBuildAggregationQuery_WithFilters(t *testing.T) {
	t.Helper()

	svc := newTestService(&mockESClient{})
	req := &domain.AggregationRequest{
		Filters: &domain.DocumentFilters{
			Sources: []string{"example_com"},
		},
	}
	aggs := map[string]any{
		"test_agg": map[string]any{"terms": map[string]any{"field": "test"}},
	}

	query := svc.buildAggregationQuery(req, aggs)

	if _, hasQuery := query["query"]; !hasQuery {
		t.Error("query should be set when filters are provided")
	}
}

func TestBuildAggregationQuery_NilFiltersInRequest(t *testing.T) {
	t.Helper()

	svc := newTestService(&mockESClient{})
	req := &domain.AggregationRequest{}
	aggs := map[string]any{}

	query := svc.buildAggregationQuery(req, aggs)

	if _, hasQuery := query["query"]; hasQuery {
		t.Error("query should not be set when filters are nil")
	}
}

// --- buildClassificationDriftQuery ---

func TestBuildClassificationDriftQuery_NoSources(t *testing.T) {
	t.Helper()

	svc := newTestService(&mockESClient{})
	aggs := map[string]any{}

	query := svc.buildClassificationDriftQuery(24, nil, aggs)

	if query["size"] != 0 {
		t.Errorf("size = %v, want 0", query["size"])
	}

	boolQ, ok := query["query"].(map[string]any)["bool"].(map[string]any)
	if !ok {
		t.Fatal("expected bool query")
	}
	filters, ok := boolQ["filter"].([]any)
	if !ok {
		t.Fatal("expected filter array")
	}
	// Only range filter, no source filter
	if len(filters) != 1 {
		t.Errorf("expected 1 filter (range only), got %d", len(filters))
	}
}

func TestBuildClassificationDriftQuery_WithSources(t *testing.T) {
	t.Helper()

	svc := newTestService(&mockESClient{})
	aggs := map[string]any{}

	query := svc.buildClassificationDriftQuery(48, []string{"src_a", "src_b"}, aggs)

	boolQ := query["query"].(map[string]any)["bool"].(map[string]any)
	filters := boolQ["filter"].([]any)
	// Range filter + source terms filter
	if len(filters) != 2 {
		t.Errorf("expected 2 filters (range + source), got %d", len(filters))
	}
}

// --- GetCrimeAggregation ---

func TestGetCrimeAggregation_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 500}},
		"aggregations": {
			"by_sub_label": {"buckets": [{"key": "robbery", "doc_count": 50}]},
			"by_relevance": {"buckets": [{"key": "core_street_crime", "doc_count": 100}]},
			"by_crime_type": {"buckets": [{"key": "theft", "doc_count": 30}]},
			"crime_related": {"doc_count": 150}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetCrimeAggregation(context.Background(), &domain.AggregationRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalDocuments != 500 {
		t.Errorf("TotalDocuments = %d, want 500", result.TotalDocuments)
	}
	if result.TotalCrimeRelated != 150 {
		t.Errorf("TotalCrimeRelated = %d, want 150", result.TotalCrimeRelated)
	}
	if result.BySubLabel["robbery"] != 50 {
		t.Errorf("BySubLabel[robbery] = %d, want 50", result.BySubLabel["robbery"])
	}
}

func TestGetCrimeAggregation_ESError(t *testing.T) {
	t.Helper()

	mock := &mockESClient{searchErr: errTestES}
	svc := newTestService(mock)

	_, err := svc.GetCrimeAggregation(context.Background(), &domain.AggregationRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- GetLocationAggregation ---

func TestGetLocationAggregation_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 200}},
		"aggregations": {
			"by_country": {"buckets": [{"key": "Canada", "doc_count": 180}]},
			"by_province": {"buckets": [{"key": "Ontario", "doc_count": 100}]},
			"by_city": {"buckets": [{"key": "Sudbury", "doc_count": 50}]},
			"by_specificity": {"buckets": [{"key": "city", "doc_count": 120}]}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetLocationAggregation(context.Background(), &domain.AggregationRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ByCountry["Canada"] != 180 {
		t.Errorf("ByCountry[Canada] = %d, want 180", result.ByCountry["Canada"])
	}
	if result.ByCity["Sudbury"] != 50 {
		t.Errorf("ByCity[Sudbury] = %d, want 50", result.ByCity["Sudbury"])
	}
}

// --- GetOverviewAggregation ---

func TestGetOverviewAggregation_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 1000}},
		"aggregations": {
			"top_cities": {"buckets": [{"key": "Toronto", "doc_count": 200}]},
			"top_crime_types": {"buckets": [{"key": "theft", "doc_count": 80}]},
			"crime_related": {"doc_count": 300},
			"quality_high": {"doc_count": 400},
			"quality_medium": {"doc_count": 350},
			"quality_low": {"doc_count": 250}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetOverviewAggregation(context.Background(), &domain.AggregationRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalDocuments != 1000 {
		t.Errorf("TotalDocuments = %d, want 1000", result.TotalDocuments)
	}
	if result.TotalCrimeRelated != 300 {
		t.Errorf("TotalCrimeRelated = %d, want 300", result.TotalCrimeRelated)
	}
	if result.QualityDistribution.High != 400 {
		t.Errorf("QualityDistribution.High = %d, want 400", result.QualityDistribution.High)
	}
}

// --- GetMiningAggregation ---

func TestGetMiningAggregation_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 300}},
		"aggregations": {
			"by_relevance": {"buckets": [{"key": "core_mining", "doc_count": 100}]},
			"by_mining_stage": {"buckets": [{"key": "exploration", "doc_count": 40}]},
			"by_commodity": {"buckets": [{"key": "gold", "doc_count": 60}]},
			"by_location": {"buckets": [{"key": "Sudbury", "doc_count": 30}]},
			"mining_related": {"doc_count": 120}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetMiningAggregation(context.Background(), &domain.AggregationRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalMining != 120 {
		t.Errorf("TotalMining = %d, want 120", result.TotalMining)
	}
	if result.ByCommodity["gold"] != 60 {
		t.Errorf("ByCommodity[gold] = %d, want 60", result.ByCommodity["gold"])
	}
}

func TestGetMiningAggregation_ESError(t *testing.T) {
	t.Helper()

	mock := &mockESClient{searchErr: errTestES}
	svc := newTestService(mock)

	_, err := svc.GetMiningAggregation(context.Background(), &domain.AggregationRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- GetClassificationDriftAggregation ---

func TestGetClassificationDriftAggregation_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 100}},
		"aggregations": {
			"by_content_type": {"buckets": [{"key": "article", "doc_count": 70}]},
			"by_crime_relevance": {"buckets": [{"key": "core_street_crime", "doc_count": 30}]},
			"content_type_x_crime": {
				"buckets": [
					{
						"key": "article",
						"doc_count": 70,
						"crime": {"buckets": [{"key": "core_street_crime", "doc_count": 25}]}
					}
				]
			}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	req := &domain.ClassificationDriftRequest{Hours: 24}
	result, err := svc.GetClassificationDriftAggregation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalDocuments != 100 {
		t.Errorf("TotalDocuments = %d, want 100", result.TotalDocuments)
	}
	if result.ByContentType["article"] != 70 {
		t.Errorf("ByContentType[article] = %d, want 70", result.ByContentType["article"])
	}
	if result.ContentTypeXCrime["article"]["core_street_crime"] != 25 {
		t.Errorf("ContentTypeXCrime[article][core_street_crime] = %d, want 25",
			result.ContentTypeXCrime["article"]["core_street_crime"])
	}
}

func TestGetClassificationDriftAggregation_DefaultHours(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {"total": {"value": 0}},
		"aggregations": {
			"by_content_type": {"buckets": []},
			"by_crime_relevance": {"buckets": []},
			"content_type_x_crime": {"buckets": []}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	req := &domain.ClassificationDriftRequest{Hours: 0} // should default to 24
	result, err := svc.GetClassificationDriftAggregation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalDocuments != 0 {
		t.Errorf("TotalDocuments = %d, want 0", result.TotalDocuments)
	}
}

// --- GetContentTypeMismatchCount ---

func TestGetContentTypeMismatchCount_Success(t *testing.T) {
	t.Helper()

	body := `{"aggregations": {"mismatch": {"doc_count": 7}}}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetContentTypeMismatchCount(context.Background(), 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 7 {
		t.Errorf("Count = %d, want 7", result.Count)
	}
}

func TestGetContentTypeMismatchCount_DefaultHours(t *testing.T) {
	t.Helper()

	body := `{"aggregations": {"mismatch": {"doc_count": 0}}}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetContentTypeMismatchCount(context.Background(), 0) // defaults to 24
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// --- GetSuspectedMisclassifications ---

func TestGetSuspectedMisclassifications_Success(t *testing.T) {
	t.Helper()

	body := `{
		"hits": {
			"total": {"value": 2},
			"hits": [
				{
					"_id": "doc1",
					"_source": {
						"title": "Crime Page",
						"canonical_url": "https://example.com/page1",
						"content_type": "page",
						"confidence": 0.85,
						"crawled_at": "2024-01-15T12:00:00Z",
						"crime": {"street_crime_relevance": "core_street_crime"}
					}
				},
				{
					"_id": "doc2",
					"_source": {
						"title": "Another Page",
						"canonical_url": "https://example.com/page2",
						"content_type": "page",
						"crawled_at": "2024-01-15T11:00:00Z"
					}
				}
			]
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetSuspectedMisclassifications(context.Background(), 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if len(result.Documents) != 2 {
		t.Fatalf("Documents count = %d, want 2", len(result.Documents))
	}
	if result.Documents[0].CrimeRelevance != "core_street_crime" {
		t.Errorf("CrimeRelevance = %q, want %q", result.Documents[0].CrimeRelevance, "core_street_crime")
	}
	if result.Documents[1].CrimeRelevance != "" {
		t.Errorf("CrimeRelevance = %q, want empty for doc without crime", result.Documents[1].CrimeRelevance)
	}
}

func TestGetSuspectedMisclassifications_DefaultHours(t *testing.T) {
	t.Helper()

	body := `{"hits": {"total": {"value": 0}, "hits": []}}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetSuspectedMisclassifications(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
}

// --- GetClassificationDriftTimeseries ---

func TestGetClassificationDriftTimeseries_Success(t *testing.T) {
	t.Helper()

	body := `{
		"aggregations": {
			"by_day": {
				"buckets": [
					{
						"key_as_string": "2024-01-14",
						"doc_count": 100,
						"by_content_type": {
							"buckets": [
								{"key": "article", "doc_count": 60},
								{"key": "page", "doc_count": 30}
							]
						}
					}
				]
			}
		}
	}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetClassificationDriftTimeseries(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Buckets) != 1 {
		t.Fatalf("Buckets length = %d, want 1", len(result.Buckets))
	}
	bucket := result.Buckets[0]
	if bucket.Date != "2024-01-14" {
		t.Errorf("Date = %q, want %q", bucket.Date, "2024-01-14")
	}
	if bucket.ArticleCount != 60 {
		t.Errorf("ArticleCount = %d, want 60", bucket.ArticleCount)
	}
	if bucket.PageCount != 30 {
		t.Errorf("PageCount = %d, want 30", bucket.PageCount)
	}
	if bucket.OtherCount != 10 {
		t.Errorf("OtherCount = %d, want 10", bucket.OtherCount)
	}
	if bucket.Total != 100 {
		t.Errorf("Total = %d, want 100", bucket.Total)
	}
}

func TestGetClassificationDriftTimeseries_DefaultDays(t *testing.T) {
	t.Helper()

	body := `{"aggregations": {"by_day": {"buckets": []}}}`
	mock := &mockESClient{searchResp: esapiResponse(t, 200, body)}
	svc := newTestService(mock)

	result, err := svc.GetClassificationDriftTimeseries(context.Background(), 0) // defaults to 7
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Buckets) != 0 {
		t.Errorf("Buckets length = %d, want 0", len(result.Buckets))
	}
}

func TestGetClassificationDriftTimeseries_ESError(t *testing.T) {
	t.Helper()

	mock := &mockESClient{searchErr: errTestES}
	svc := newTestService(mock)

	_, err := svc.GetClassificationDriftTimeseries(context.Background(), 7)
	if err == nil {
		t.Fatal("expected error")
	}
}

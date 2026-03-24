package elasticsearch //nolint:testpackage // testing unexported validation/query-building functions

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
)

// --- Validation ---

func TestValidateRequest_Defaults(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("validateRequest() unexpected error: %v", err)
	}

	if req.Pagination == nil {
		t.Fatal("pagination should be set to default")
	}
	if req.Pagination.Page != 1 {
		t.Errorf("default page = %d, want 1", req.Pagination.Page)
	}
	if req.Pagination.Size != defaultPageSize {
		t.Errorf("default size = %d, want %d", req.Pagination.Size, defaultPageSize)
	}
	if req.Sort == nil {
		t.Fatal("sort should be set to default")
	}
	if req.Sort.Field != "relevance" {
		t.Errorf("default sort field = %q, want %q", req.Sort.Field, "relevance")
	}
	if req.Sort.Order != "desc" {
		t.Errorf("default sort order = %q, want %q", req.Sort.Order, "desc")
	}
}

func TestValidateRequest_PageSizeExceedsMax(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Pagination: &domain.DocumentPagination{
			Page: 1,
			Size: 200,
		},
	}

	err := qb.validateRequest(req)
	if err == nil {
		t.Fatal("validateRequest() should error when size > maxPageSize")
	}
}

func TestValidateRequest_NegativePageCorrected(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Pagination: &domain.DocumentPagination{
			Page: -1,
			Size: 10,
		},
	}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("validateRequest() unexpected error: %v", err)
	}

	if req.Pagination.Page != 1 {
		t.Errorf("negative page corrected to %d, want 1", req.Pagination.Page)
	}
}

func TestValidateRequest_InvalidSortFieldCorrected(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Sort: &domain.DocumentSort{
			Field: "invalid_field",
			Order: "asc",
		},
	}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("validateRequest() unexpected error: %v", err)
	}

	if req.Sort.Field != "relevance" {
		t.Errorf("invalid sort field corrected to %q, want %q", req.Sort.Field, "relevance")
	}
}

func TestValidateRequest_InvalidSortOrderCorrected(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Sort: &domain.DocumentSort{
			Field: "title",
			Order: "invalid",
		},
	}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("validateRequest() unexpected error: %v", err)
	}

	if req.Sort.Order != "desc" {
		t.Errorf("invalid sort order corrected to %q, want %q", req.Sort.Order, "desc")
	}
}

func TestValidateRequest_ValidSortFields(t *testing.T) {
	t.Helper()

	validFields := []string{"relevance", "published_date", "crawled_at", "quality_score", "title"}
	qb := NewDocumentQueryBuilder()

	for _, field := range validFields {
		t.Run(field, func(t *testing.T) {
			req := &domain.DocumentQueryRequest{
				Sort: &domain.DocumentSort{
					Field: field,
					Order: "asc",
				},
			}
			if err := qb.validateRequest(req); err != nil {
				t.Fatalf("validateRequest() with sort field %q: %v", field, err)
			}
			if req.Sort.Field != field {
				t.Errorf("sort field changed from %q to %q", field, req.Sort.Field)
			}
		})
	}
}

func TestValidateRequest_QualityScoreRange(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()

	// Negative min quality score should error
	req := &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			MinQualityScore: -1,
		},
	}
	if err := qb.validateRequest(req); err == nil {
		t.Error("validateRequest() should error for negative min_quality_score")
	}

	// Min > max should error
	req = &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			MinQualityScore: 80,
			MaxQualityScore: 50,
		},
	}
	if err := qb.validateRequest(req); err == nil {
		t.Error("validateRequest() should error when min > max quality score")
	}
}

func TestValidateRequest_DateRangeValidation(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	now := time.Now()
	past := now.Add(-24 * time.Hour)

	// from_date after to_date should error
	req := &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			FromDate: &now,
			ToDate:   &past,
		},
	}
	if err := qb.validateRequest(req); err == nil {
		t.Error("validateRequest() should error when from_date > to_date")
	}

	// from_crawled_at after to_crawled_at should error
	req = &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			FromCrawledAt: &now,
			ToCrawledAt:   &past,
		},
	}
	if err := qb.validateRequest(req); err == nil {
		t.Error("validateRequest() should error when from_crawled_at > to_crawled_at")
	}
}

// --- Build ---

func TestBuild_BasicQuery(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Query: "test search",
		Pagination: &domain.DocumentPagination{
			Page: 2,
			Size: 10,
		},
	}

	query, err := qb.Build(req)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Check pagination offset: (page-1) * size = (2-1) * 10 = 10
	if query["from"] != 10 {
		t.Errorf("from = %v, want 10", query["from"])
	}
	if query["size"] != 10 {
		t.Errorf("size = %v, want 10", query["size"])
	}
	if query["track_total_hits"] != true {
		t.Error("track_total_hits should be true")
	}
}

func TestBuild_EmptyQuery(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{}

	query, err := qb.Build(req)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if query == nil {
		t.Fatal("Build() returned nil query")
	}
}

// --- Sort Building ---

func TestBuildSort_Relevance(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	sort := qb.buildSort(&domain.DocumentSort{Field: "relevance", Order: "desc"})

	if len(sort) != 1 {
		t.Fatalf("sort clauses = %d, want 1", len(sort))
	}
	if _, ok := sort[0]["_score"]; !ok {
		t.Error("relevance sort should use _score")
	}
}

func TestBuildSort_DateFieldsHaveMissing(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()

	dateFields := []string{"published_date", "crawled_at"}
	for _, field := range dateFields {
		t.Run(field, func(t *testing.T) {
			sort := qb.buildSort(&domain.DocumentSort{Field: field, Order: "asc"})
			if len(sort) != 1 {
				t.Fatalf("sort clauses = %d, want 1", len(sort))
			}
			fieldSort, ok := sort[0][field].(map[string]any)
			if !ok {
				t.Fatalf("sort clause missing %q field", field)
			}
			if fieldSort["missing"] != "_last" {
				t.Errorf("missing = %v, want _last", fieldSort["missing"])
			}
		})
	}
}

func TestBuildSort_NilDefaultsToRelevance(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	sort := qb.buildSort(nil)

	if len(sort) != 1 {
		t.Fatalf("sort clauses = %d, want 1", len(sort))
	}
	scoreSort, ok := sort[0]["_score"].(map[string]any)
	if !ok {
		t.Fatal("nil sort should default to _score")
	}
	if scoreSort["order"] != "desc" {
		t.Errorf("default sort order = %v, want desc", scoreSort["order"])
	}
}

// --- Filter Building ---

func TestBuildFilters_ContentType(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{
		ContentType: "article",
	}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_CrimeRelevance(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{
		CrimeRelevance: []string{"core_street_crime"},
	}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_LocationFilters(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{
		Cities:    []string{"Sudbury"},
		Provinces: []string{"Ontario"},
	}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 2 {
		t.Fatalf("filter count = %d, want 2", len(result))
	}
}

func TestBuildFilters_Nil(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	result := qb.BuildFiltersOnly(nil)

	if len(result) != 0 {
		t.Errorf("nil filters should return empty, got %d", len(result))
	}
}

// --- Additional filter coverage ---

func TestBuildFilters_TitleFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{Title: "Breaking News"}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_URLFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{URL: "example.com"}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_QualityScoreRange(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()

	// Min only
	filters := &domain.DocumentFilters{MinQualityScore: 50}
	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("min quality filter count = %d, want 1", len(result))
	}

	// Max only (below 100)
	filters = &domain.DocumentFilters{MaxQualityScore: 80}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("max quality filter count = %d, want 1", len(result))
	}

	// Both min and max
	filters = &domain.DocumentFilters{MinQualityScore: 40, MaxQualityScore: 80}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("min+max quality filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_QualityScoreMaxAt100_NoFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	// MaxQualityScore at 100 (maxQualityScore) should not generate a filter
	filters := &domain.DocumentFilters{MaxQualityScore: maxQualityScore}
	result := qb.BuildFiltersOnly(filters)
	if len(result) != 0 {
		t.Fatalf("max=100 should not generate filter, got %d", len(result))
	}
}

func TestBuildFilters_TopicsFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{Topics: []string{"crime", "local"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("topics filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_DateRanges(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	now := time.Now()
	past := now.Add(-48 * time.Hour)

	// Published date range
	filters := &domain.DocumentFilters{FromDate: &past, ToDate: &now}
	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("published date filter count = %d, want 1", len(result))
	}

	// Crawled at range
	filters = &domain.DocumentFilters{FromCrawledAt: &past, ToCrawledAt: &now}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("crawled at filter count = %d, want 1", len(result))
	}

	// From date only
	filters = &domain.DocumentFilters{FromDate: &past}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("from date only filter count = %d, want 1", len(result))
	}

	// To date only
	filters = &domain.DocumentFilters{ToDate: &now}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("to date only filter count = %d, want 1", len(result))
	}

	// From crawled at only
	filters = &domain.DocumentFilters{FromCrawledAt: &past}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("from crawled_at only filter count = %d, want 1", len(result))
	}

	// To crawled at only
	filters = &domain.DocumentFilters{ToCrawledAt: &now}
	result = qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("to crawled_at only filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_CrimeSubLabels(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{CrimeSubLabels: []string{"robbery", "assault"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("crime sub labels filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_CrimeTypes(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{CrimeTypes: []string{"theft", "fraud"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("crime types filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_HomepageEligible(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	boolTrue := true
	filters := &domain.DocumentFilters{HomepageEligible: &boolTrue}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("homepage eligible filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_ReviewRequired(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	boolFalse := false
	filters := &domain.DocumentFilters{ReviewRequired: &boolFalse}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("review required filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_CountriesFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{Countries: []string{"Canada", "USA"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("countries filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_SpecificityFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{Specificity: []string{"city", "province"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("specificity filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_SourcesFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	filters := &domain.DocumentFilters{Sources: []string{"example_com", "news_ca"}}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("sources filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_AllFiltersCombined(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	now := time.Now()
	past := now.Add(-24 * time.Hour)
	boolTrue := true

	filters := &domain.DocumentFilters{
		Title:            "test",
		URL:              "example.com",
		ContentType:      "article",
		MinQualityScore:  50,
		MaxQualityScore:  90,
		Topics:           []string{"crime"},
		FromDate:         &past,
		ToDate:           &now,
		FromCrawledAt:    &past,
		ToCrawledAt:      &now,
		CrimeRelevance:   []string{"core_street_crime"},
		CrimeSubLabels:   []string{"robbery"},
		CrimeTypes:       []string{"theft"},
		HomepageEligible: &boolTrue,
		ReviewRequired:   &boolTrue,
		Cities:           []string{"Sudbury"},
		Provinces:        []string{"Ontario"},
		Countries:        []string{"Canada"},
		Specificity:      []string{"city"},
		Sources:          []string{"example_com"},
	}

	result := qb.BuildFiltersOnly(filters)
	// title, url, content_type, quality, topics, published_date, crawled_at,
	// crime_relevance, crime_sub_labels, crime_types, homepage_eligible, review_required,
	// cities, provinces, countries, specificity, sources = 17
	expectedCount := 17
	if len(result) != expectedCount {
		t.Errorf("combined filter count = %d, want %d", len(result), expectedCount)
	}
}

// --- Additional sort coverage ---

func TestBuildSort_QualityScore(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	sort := qb.buildSort(&domain.DocumentSort{Field: "quality_score", Order: "asc"})

	if len(sort) != 1 {
		t.Fatalf("sort clauses = %d, want 1", len(sort))
	}
	fieldSort, ok := sort[0]["quality_score"].(map[string]any)
	if !ok {
		t.Fatal("sort clause missing quality_score field")
	}
	if fieldSort["order"] != "asc" {
		t.Errorf("order = %v, want asc", fieldSort["order"])
	}
	if fieldSort["missing"] != "_last" {
		t.Errorf("missing = %v, want _last", fieldSort["missing"])
	}
}

func TestBuildSort_Title(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	sort := qb.buildSort(&domain.DocumentSort{Field: "title", Order: "asc"})

	if len(sort) != 1 {
		t.Fatalf("sort clauses = %d, want 1", len(sort))
	}
	fieldSort, ok := sort[0]["title.keyword"].(map[string]any)
	if !ok {
		t.Fatal("sort clause missing title.keyword field")
	}
	if fieldSort["order"] != "asc" {
		t.Errorf("order = %v, want asc", fieldSort["order"])
	}
}

func TestBuildSort_UnknownFieldDefaultsToRelevance(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	sort := qb.buildSort(&domain.DocumentSort{Field: "unknown_field", Order: "desc"})

	if len(sort) != 1 {
		t.Fatalf("sort clauses = %d, want 1", len(sort))
	}
	if _, ok := sort[0]["_score"]; !ok {
		t.Error("unknown sort field should default to _score")
	}
}

// --- Build with query text ---

func TestBuild_WithQueryText_HasMultiMatch(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Query: "search terms",
	}

	query, err := qb.Build(req)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Verify query contains a bool query with must clause
	queryField, ok := query["query"].(map[string]any)
	if !ok {
		t.Fatal("query field should be a map")
	}
	boolField, ok := queryField["bool"].(map[string]any)
	if !ok {
		t.Fatal("bool field should be a map")
	}
	must, ok := boolField["must"].([]any)
	if !ok {
		t.Fatal("must field should be an array")
	}
	if len(must) != 1 {
		t.Errorf("must clause count = %d, want 1", len(must))
	}
}

func TestBuild_WithWhitespaceOnlyQuery_NoMultiMatch(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Query: "   ",
	}

	query, err := qb.Build(req)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	queryField := query["query"].(map[string]any)
	boolField := queryField["bool"].(map[string]any)
	must := boolField["must"].([]any)
	if len(must) != 0 {
		t.Errorf("whitespace-only query should have empty must, got %d", len(must))
	}
}

func TestBuild_WithFilters(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			ContentType: "article",
			Sources:     []string{"example_com"},
		},
	}

	query, err := qb.Build(req)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	queryField := query["query"].(map[string]any)
	boolField := queryField["bool"].(map[string]any)
	filterField, ok := boolField["filter"].([]any)
	if !ok {
		t.Fatal("filter field should be an array")
	}
	if len(filterField) != 2 {
		t.Errorf("filter count = %d, want 2", len(filterField))
	}
}

// --- Validate request edge cases ---

func TestValidateRequest_ZeroSizeCorrected(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Pagination: &domain.DocumentPagination{Page: 1, Size: 0},
	}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Pagination.Size != defaultPageSize {
		t.Errorf("zero size corrected to %d, want %d", req.Pagination.Size, defaultPageSize)
	}
}

func TestValidateRequest_MaxQualityScoreOverMax_Corrected(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	req := &domain.DocumentQueryRequest{
		Filters: &domain.DocumentFilters{
			MaxQualityScore: 200,
		},
	}

	if err := qb.validateRequest(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Filters.MaxQualityScore != maxQualityScore {
		t.Errorf("max quality score corrected to %d, want %d", req.Filters.MaxQualityScore, maxQualityScore)
	}
}

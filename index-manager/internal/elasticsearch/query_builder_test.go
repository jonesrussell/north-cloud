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

func TestBuildFilters_LegacyCrimeRelated(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	trueVal := true
	filters := &domain.DocumentFilters{
		IsCrimeRelated: &trueVal,
	}

	result := qb.BuildFiltersOnly(filters)
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1", len(result))
	}
}

func TestBuildFilters_LegacyCrimeNotUsedWithNewFilter(t *testing.T) {
	t.Helper()

	qb := NewDocumentQueryBuilder()
	trueVal := true
	filters := &domain.DocumentFilters{
		IsCrimeRelated: &trueVal,
		CrimeRelevance: []string{"core_street_crime"},
	}

	result := qb.BuildFiltersOnly(filters)
	// Should only have new filter, not legacy
	if len(result) != 1 {
		t.Fatalf("filter count = %d, want 1 (new filter only)", len(result))
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

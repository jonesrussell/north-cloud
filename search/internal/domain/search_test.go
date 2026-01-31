package domain_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

const (
	testMaxPageSize     = 100
	testDefaultPageSize = 20
	testMaxQueryLength  = 500
)

func TestSearchRequest_Validate_Defaults(t *testing.T) {
	t.Helper()

	req := &domain.SearchRequest{
		Query: "test query",
	}

	err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	// Check pagination defaults
	if req.Pagination == nil {
		t.Fatal("Validate() should set default pagination")
	}
	if req.Pagination.Page != 1 {
		t.Errorf("Validate() page = %d, want 1", req.Pagination.Page)
	}
	if req.Pagination.Size != testDefaultPageSize {
		t.Errorf("Validate() size = %d, want %d", req.Pagination.Size, testDefaultPageSize)
	}

	// Check filters defaults
	if req.Filters == nil {
		t.Fatal("Validate() should set default filters")
	}
	if req.Filters.MaxQualityScore != 100 {
		t.Errorf("Validate() max_quality_score = %d, want 100", req.Filters.MaxQualityScore)
	}

	// Check sort defaults
	if req.Sort == nil {
		t.Fatal("Validate() should set default sort")
	}
	if req.Sort.Field != "relevance" {
		t.Errorf("Validate() sort field = %s, want relevance", req.Sort.Field)
	}
	if req.Sort.Order != "desc" {
		t.Errorf("Validate() sort order = %s, want desc", req.Sort.Order)
	}

	// Check options defaults
	if req.Options == nil {
		t.Fatal("Validate() should set default options")
	}
	if !req.Options.IncludeHighlights {
		t.Error("Validate() should enable highlights by default")
	}
	if !req.Options.IncludeFacets {
		t.Error("Validate() should enable facets by default")
	}
}

func TestSearchRequest_Validate_QueryLength(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		queryLen  int
		maxLen    int
		wantError bool
	}{
		{"within limit", 100, 500, false},
		{"at limit", 500, 500, false},
		{"exceeds limit", 501, 500, true},
		{"empty query", 0, 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := make([]byte, tt.queryLen)
			for i := range query {
				query[i] = 'a'
			}

			req := &domain.SearchRequest{Query: string(query)}
			err := req.Validate(testMaxPageSize, testDefaultPageSize, tt.maxLen)

			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestSearchRequest_Validate_Pagination(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		page     int
		size     int
		wantPage int
		wantSize int
		wantErr  bool
	}{
		{"valid pagination", 2, 25, 2, 25, false},
		{"zero page (corrected)", 0, 20, 1, 20, false},
		{"negative page (corrected)", -5, 20, 1, 20, false},
		{"zero size (corrected)", 1, 0, 1, testDefaultPageSize, false},
		{"size at limit", 1, testMaxPageSize, 1, testMaxPageSize, false},
		{"size exceeds limit", 1, testMaxPageSize + 1, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Pagination: &domain.Pagination{
					Page: tt.page,
					Size: tt.size,
				},
			}

			err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if req.Pagination.Page != tt.wantPage {
					t.Errorf("Validate() page = %d, want %d", req.Pagination.Page, tt.wantPage)
				}
				if req.Pagination.Size != tt.wantSize {
					t.Errorf("Validate() size = %d, want %d", req.Pagination.Size, tt.wantSize)
				}
			}
		})
	}
}

func TestSearchRequest_Validate_QualityScoreFilters(t *testing.T) {
	t.Helper()

	tests := []struct {
		name         string
		minScore     int
		maxScore     int
		wantMaxScore int
		wantErr      bool
	}{
		{"valid range", 20, 80, 80, false},
		{"min equals max", 50, 50, 50, false},
		{"zero max (defaults to 100)", 10, 0, 100, false},
		{"negative min", -10, 80, 0, true},
		{"min exceeds 100", 110, 100, 0, true},
		{"min exceeds max", 80, 50, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Filters: &domain.Filters{
					MinQualityScore: tt.minScore,
					MaxQualityScore: tt.maxScore,
				},
			}

			err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && req.Filters.MaxQualityScore != tt.wantMaxScore {
				t.Errorf("Validate() maxQualityScore = %d, want %d",
					req.Filters.MaxQualityScore, tt.wantMaxScore)
			}
		})
	}
}

func TestSearchRequest_Validate_DateFilters(t *testing.T) {
	t.Helper()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name     string
		fromDate *time.Time
		toDate   *time.Time
		wantErr  bool
	}{
		{"no dates", nil, nil, false},
		{"from date only", &yesterday, nil, false},
		{"to date only", nil, &tomorrow, false},
		{"valid range", &yesterday, &tomorrow, false},
		{"same date", &now, &now, false},
		{"from after to", &tomorrow, &yesterday, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Filters: &domain.Filters{
					FromDate: tt.fromDate,
					ToDate:   tt.toDate,
				},
			}

			err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchRequest_Validate_SortField(t *testing.T) {
	t.Helper()

	validFields := []string{"relevance", "published_date", "quality_score", "crawled_at"}

	for _, field := range validFields {
		t.Run(field, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Sort: &domain.Sort{
					Field: field,
					Order: "desc",
				},
			}

			err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if req.Sort.Field != field {
				t.Errorf("Validate() sort field = %s, want %s", req.Sort.Field, field)
			}
		})
	}

	// Invalid field defaults to relevance
	t.Run("invalid field defaults to relevance", func(t *testing.T) {
		req := &domain.SearchRequest{
			Query: "test",
			Sort: &domain.Sort{
				Field: "invalid_field",
				Order: "desc",
			},
		}

		err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		if req.Sort.Field != "relevance" {
			t.Errorf("Validate() sort field = %s, want relevance", req.Sort.Field)
		}
	})
}

func TestSearchRequest_Validate_SortOrder(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		order     string
		wantOrder string
	}{
		{"asc", "asc", "asc"},
		{"desc", "desc", "desc"},
		{"invalid defaults to desc", "invalid", "desc"},
		{"empty defaults to desc", "", "desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.SearchRequest{
				Query: "test",
				Sort: &domain.Sort{
					Field: "relevance",
					Order: tt.order,
				},
			}

			err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if req.Sort.Order != tt.wantOrder {
				t.Errorf("Validate() sort order = %s, want %s", req.Sort.Order, tt.wantOrder)
			}
		})
	}
}

func TestSearchRequest_Validate_PreservesValidOptions(t *testing.T) {
	t.Helper()

	req := &domain.SearchRequest{
		Query: "test",
		Options: &domain.Options{
			IncludeHighlights: false,
			IncludeFacets:     false,
			SourceFields:      []string{"title", "url"},
		},
	}

	err := req.Validate(testMaxPageSize, testDefaultPageSize, testMaxQueryLength)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	// Options should be preserved, not overwritten
	if req.Options.IncludeHighlights {
		t.Error("Validate() should preserve IncludeHighlights=false")
	}
	if req.Options.IncludeFacets {
		t.Error("Validate() should preserve IncludeFacets=false")
	}
	if len(req.Options.SourceFields) != 2 {
		t.Errorf("Validate() should preserve SourceFields, got %d fields", len(req.Options.SourceFields))
	}
}

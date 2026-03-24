//nolint:testpackage // tests unexported parseFilters function
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func newTestContext(rawQuery string) *gin.Context {
	t := &testing.T{}
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/?"+rawQuery, http.NoBody)
	c.Request = req

	return c
}

// ---------------------------------------------------------------------------
// parseFilters
// ---------------------------------------------------------------------------

func TestParseFilters_TopicsArrayFormat(t *testing.T) {
	t.Helper()

	c := newTestContext("topics%5B%5D=indigenous&topics%5B%5D=crime")
	filters := parseFilters(c)

	if len(filters.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d: %v", len(filters.Topics), filters.Topics)
	}
	if filters.Topics[0] != "indigenous" {
		t.Errorf("expected topics[0]=%q, got %q", "indigenous", filters.Topics[0])
	}
	if filters.Topics[1] != "crime" {
		t.Errorf("expected topics[1]=%q, got %q", "crime", filters.Topics[1])
	}
}

func TestParseFilters_TopicsCommaFormat(t *testing.T) {
	t.Helper()

	c := newTestContext("topics=indigenous,crime")
	filters := parseFilters(c)

	if len(filters.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d: %v", len(filters.Topics), filters.Topics)
	}
	if filters.Topics[0] != "indigenous" {
		t.Errorf("expected topics[0]=%q, got %q", "indigenous", filters.Topics[0])
	}
	if filters.Topics[1] != "crime" {
		t.Errorf("expected topics[1]=%q, got %q", "crime", filters.Topics[1])
	}
}

func TestParseFilters_TopicsEmpty(t *testing.T) {
	t.Helper()

	c := newTestContext("q=technology")
	filters := parseFilters(c)

	if len(filters.Topics) != 0 {
		t.Errorf("expected no topics, got %v", filters.Topics)
	}
}

func TestParseFilters_ContentType(t *testing.T) {
	t.Helper()

	c := newTestContext("content_type=article")
	filters := parseFilters(c)

	if filters.ContentType != "article" {
		t.Errorf("expected content_type=%q, got %q", "article", filters.ContentType)
	}
}

func TestParseFilters_MinQuality(t *testing.T) {
	t.Helper()

	c := newTestContext("min_quality=60")
	filters := parseFilters(c)

	if filters.MinQualityScore != 60 {
		t.Errorf("expected min_quality_score=60, got %d", filters.MinQualityScore)
	}
}

func TestParseFilters_MinQualityInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("min_quality=abc")
	filters := parseFilters(c)

	if filters.MinQualityScore != 0 {
		t.Errorf("expected min_quality_score=0 for invalid input, got %d", filters.MinQualityScore)
	}
}

func TestParseFilters_MaxQuality(t *testing.T) {
	t.Helper()

	c := newTestContext("max_quality=80")
	filters := parseFilters(c)

	if filters.MaxQualityScore != 80 {
		t.Errorf("expected max_quality_score=80, got %d", filters.MaxQualityScore)
	}
}

func TestParseFilters_MaxQualityInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("max_quality=xyz")
	filters := parseFilters(c)

	if filters.MaxQualityScore != 0 {
		t.Errorf("expected max_quality_score=0 for invalid input, got %d", filters.MaxQualityScore)
	}
}

func TestParseFilters_CrimeRelevance(t *testing.T) {
	t.Helper()

	c := newTestContext("crime_relevance=high,medium")
	filters := parseFilters(c)

	if len(filters.CrimeRelevance) != 2 {
		t.Fatalf("expected 2 crime_relevance values, got %d", len(filters.CrimeRelevance))
	}
	if filters.CrimeRelevance[0] != "high" {
		t.Errorf("expected crime_relevance[0]=%q, got %q", "high", filters.CrimeRelevance[0])
	}
}

func TestParseFilters_Sources(t *testing.T) {
	t.Helper()

	c := newTestContext("sources=cbc,ctv")
	filters := parseFilters(c)

	if len(filters.SourceNames) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(filters.SourceNames))
	}
	if filters.SourceNames[0] != "cbc" {
		t.Errorf("expected sources[0]=%q, got %q", "cbc", filters.SourceNames[0])
	}
}

func TestParseFilters_FromDate(t *testing.T) {
	t.Helper()

	c := newTestContext("from_date=2025-01-15")
	filters := parseFilters(c)

	if filters.FromDate == nil {
		t.Fatal("expected from_date to be set")
	}
	if filters.FromDate.Year() != 2025 || filters.FromDate.Month() != 1 || filters.FromDate.Day() != 15 {
		t.Errorf("expected from_date=2025-01-15, got %v", filters.FromDate)
	}
}

func TestParseFilters_FromDateInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("from_date=not-a-date")
	filters := parseFilters(c)

	if filters.FromDate != nil {
		t.Errorf("expected from_date=nil for invalid input, got %v", filters.FromDate)
	}
}

func TestParseFilters_ToDate(t *testing.T) {
	t.Helper()

	c := newTestContext("to_date=2025-06-30")
	filters := parseFilters(c)

	if filters.ToDate == nil {
		t.Fatal("expected to_date to be set")
	}
	if filters.ToDate.Year() != 2025 || filters.ToDate.Month() != 6 || filters.ToDate.Day() != 30 {
		t.Errorf("expected to_date=2025-06-30, got %v", filters.ToDate)
	}
}

func TestParseFilters_ToDateInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("to_date=invalid")
	filters := parseFilters(c)

	if filters.ToDate != nil {
		t.Errorf("expected to_date=nil for invalid input, got %v", filters.ToDate)
	}
}

// ---------------------------------------------------------------------------
// parseRfpFilters
// ---------------------------------------------------------------------------

func TestParseRfpFilters_Province(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_province=ON")
	filters := parseFilters(c)

	if filters.RfpProvince != "ON" {
		t.Errorf("expected rfp_province=%q, got %q", "ON", filters.RfpProvince)
	}
}

func TestParseRfpFilters_Sector(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_sector=IT,Healthcare")
	filters := parseFilters(c)

	if len(filters.RfpSector) != 2 {
		t.Fatalf("expected 2 rfp_sector values, got %d", len(filters.RfpSector))
	}
	if filters.RfpSector[0] != "IT" {
		t.Errorf("expected rfp_sector[0]=%q, got %q", "IT", filters.RfpSector[0])
	}
}

func TestParseRfpFilters_ClosingAfter(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_closing_after=2025-12-01")
	filters := parseFilters(c)

	if filters.RfpClosingAfter != "2025-12-01" {
		t.Errorf("expected rfp_closing_after=%q, got %q", "2025-12-01", filters.RfpClosingAfter)
	}
}

func TestParseRfpFilters_BudgetMin(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_budget_min=10000.50")
	filters := parseFilters(c)

	if filters.RfpBudgetMin == nil {
		t.Fatal("expected rfp_budget_min to be set")
	}
	if *filters.RfpBudgetMin != 10000.50 {
		t.Errorf("expected rfp_budget_min=10000.50, got %f", *filters.RfpBudgetMin)
	}
}

func TestParseRfpFilters_BudgetMinInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_budget_min=notanumber")
	filters := parseFilters(c)

	if filters.RfpBudgetMin != nil {
		t.Errorf("expected rfp_budget_min=nil for invalid input, got %v", filters.RfpBudgetMin)
	}
}

func TestParseRfpFilters_BudgetMax(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_budget_max=50000")
	filters := parseFilters(c)

	if filters.RfpBudgetMax == nil {
		t.Fatal("expected rfp_budget_max to be set")
	}
	if *filters.RfpBudgetMax != 50000 {
		t.Errorf("expected rfp_budget_max=50000, got %f", *filters.RfpBudgetMax)
	}
}

func TestParseRfpFilters_BudgetMaxInvalid(t *testing.T) {
	t.Helper()

	c := newTestContext("rfp_budget_max=abc")
	filters := parseFilters(c)

	if filters.RfpBudgetMax != nil {
		t.Errorf("expected rfp_budget_max=nil for invalid input, got %v", filters.RfpBudgetMax)
	}
}

// ---------------------------------------------------------------------------
// parsePagination
// ---------------------------------------------------------------------------

func TestParsePagination_Defaults(t *testing.T) {
	t.Helper()

	c := newTestContext("")
	pagination := parsePagination(c)

	if pagination.Page != 0 {
		t.Errorf("expected page=0 (unset), got %d", pagination.Page)
	}
	if pagination.Size != 0 {
		t.Errorf("expected size=0 (unset), got %d", pagination.Size)
	}
}

func TestParsePagination_ValidValues(t *testing.T) {
	t.Helper()

	c := newTestContext("page=3&size=50")
	pagination := parsePagination(c)

	if pagination.Page != 3 {
		t.Errorf("expected page=3, got %d", pagination.Page)
	}
	if pagination.Size != 50 {
		t.Errorf("expected size=50, got %d", pagination.Size)
	}
}

func TestParsePagination_InvalidPage(t *testing.T) {
	t.Helper()

	c := newTestContext("page=abc&size=10")
	pagination := parsePagination(c)

	if pagination.Page != 0 {
		t.Errorf("expected page=0 for invalid input, got %d", pagination.Page)
	}
	if pagination.Size != 10 {
		t.Errorf("expected size=10, got %d", pagination.Size)
	}
}

func TestParsePagination_InvalidSize(t *testing.T) {
	t.Helper()

	c := newTestContext("page=2&size=xyz")
	pagination := parsePagination(c)

	if pagination.Page != 2 {
		t.Errorf("expected page=2, got %d", pagination.Page)
	}
	if pagination.Size != 0 {
		t.Errorf("expected size=0 for invalid input, got %d", pagination.Size)
	}
}

// ---------------------------------------------------------------------------
// parseSort
// ---------------------------------------------------------------------------

func TestParseSort_Defaults(t *testing.T) {
	t.Helper()

	c := newTestContext("")
	sort := parseSort(c)

	if sort.Field != "" {
		t.Errorf("expected empty field, got %q", sort.Field)
	}
	if sort.Order != "" {
		t.Errorf("expected empty order, got %q", sort.Order)
	}
}

func TestParseSort_ValidValues(t *testing.T) {
	t.Helper()

	c := newTestContext("sort=published_date&order=asc")
	sort := parseSort(c)

	if sort.Field != "published_date" {
		t.Errorf("expected field=%q, got %q", "published_date", sort.Field)
	}
	if sort.Order != "asc" {
		t.Errorf("expected order=%q, got %q", "asc", sort.Order)
	}
}

func TestParseSort_FieldOnly(t *testing.T) {
	t.Helper()

	c := newTestContext("sort=quality_score")
	sort := parseSort(c)

	if sort.Field != "quality_score" {
		t.Errorf("expected field=%q, got %q", "quality_score", sort.Field)
	}
	if sort.Order != "" {
		t.Errorf("expected empty order, got %q", sort.Order)
	}
}

// ---------------------------------------------------------------------------
// parseOptions
// ---------------------------------------------------------------------------

func TestParseOptions_Defaults(t *testing.T) {
	t.Helper()

	c := newTestContext("")
	options := parseOptions(c)

	if options.IncludeHighlights {
		t.Error("expected IncludeHighlights=false by default")
	}
	if options.IncludeFacets {
		t.Error("expected IncludeFacets=false by default")
	}
}

func TestParseOptions_BothTrue(t *testing.T) {
	t.Helper()

	c := newTestContext("highlights=true&facets=true")
	options := parseOptions(c)

	if !options.IncludeHighlights {
		t.Error("expected IncludeHighlights=true")
	}
	if !options.IncludeFacets {
		t.Error("expected IncludeFacets=true")
	}
}

func TestParseOptions_NonTrueValue(t *testing.T) {
	t.Helper()

	c := newTestContext("highlights=yes&facets=1")
	options := parseOptions(c)

	if options.IncludeHighlights {
		t.Error("expected IncludeHighlights=false for 'yes'")
	}
	if options.IncludeFacets {
		t.Error("expected IncludeFacets=false for '1'")
	}
}

// ---------------------------------------------------------------------------
// parseQueryParams (full integration of parse* functions)
// ---------------------------------------------------------------------------

func TestParseQueryParams_FullQuery(t *testing.T) {
	t.Helper()

	c := newTestContext(
		"q=northern+mining&topics=mining,indigenous&content_type=article" +
			"&min_quality=50&page=2&size=25&sort=published_date&order=desc" +
			"&highlights=true&facets=true&sources=cbc",
	)

	h := &Handler{}
	req := h.parseQueryParams(c)

	if req.Query != "northern mining" {
		t.Errorf("expected query=%q, got %q", "northern mining", req.Query)
	}
	if req.Filters == nil {
		t.Fatal("expected filters to be set")
	}
	if len(req.Filters.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(req.Filters.Topics))
	}
	if req.Filters.ContentType != "article" {
		t.Errorf("expected content_type=%q, got %q", "article", req.Filters.ContentType)
	}
	if req.Filters.MinQualityScore != 50 {
		t.Errorf("expected min_quality=50, got %d", req.Filters.MinQualityScore)
	}
	if req.Pagination == nil {
		t.Fatal("expected pagination to be set")
	}
	if req.Pagination.Page != 2 {
		t.Errorf("expected page=2, got %d", req.Pagination.Page)
	}
	if req.Pagination.Size != 25 {
		t.Errorf("expected size=25, got %d", req.Pagination.Size)
	}
	if req.Sort == nil {
		t.Fatal("expected sort to be set")
	}
	if req.Sort.Field != "published_date" {
		t.Errorf("expected sort field=%q, got %q", "published_date", req.Sort.Field)
	}
	if req.Options == nil {
		t.Fatal("expected options to be set")
	}
	if !req.Options.IncludeHighlights {
		t.Error("expected IncludeHighlights=true")
	}
	if !req.Options.IncludeFacets {
		t.Error("expected IncludeFacets=true")
	}
}

func TestParseQueryParams_EmptyQuery(t *testing.T) {
	t.Helper()

	c := newTestContext("")
	h := &Handler{}
	req := h.parseQueryParams(c)

	if req.Query != "" {
		t.Errorf("expected empty query, got %q", req.Query)
	}
	if req.Filters == nil {
		t.Fatal("expected filters to be non-nil")
	}
	if req.Pagination == nil {
		t.Fatal("expected pagination to be non-nil")
	}
	if req.Sort == nil {
		t.Fatal("expected sort to be non-nil")
	}
	if req.Options == nil {
		t.Fatal("expected options to be non-nil")
	}
}

// ---------------------------------------------------------------------------
// Combined filter test (all filter fields in one query)
// ---------------------------------------------------------------------------

func TestParseFilters_AllFields(t *testing.T) {
	t.Helper()

	c := newTestContext(
		"topics=mining&content_type=article&min_quality=40&max_quality=90" +
			"&crime_relevance=high&sources=cbc,ctv" +
			"&from_date=2025-01-01&to_date=2025-12-31" +
			"&rfp_province=ON&rfp_sector=IT&rfp_closing_after=2025-06-01" +
			"&rfp_budget_min=5000&rfp_budget_max=100000",
	)
	filters := parseFilters(c)

	if len(filters.Topics) != 1 || filters.Topics[0] != "mining" {
		t.Errorf("topics mismatch: %v", filters.Topics)
	}
	if filters.ContentType != "article" {
		t.Errorf("content_type=%q, want article", filters.ContentType)
	}
	if filters.MinQualityScore != 40 {
		t.Errorf("min_quality=%d, want 40", filters.MinQualityScore)
	}
	if filters.MaxQualityScore != 90 {
		t.Errorf("max_quality=%d, want 90", filters.MaxQualityScore)
	}
	if len(filters.CrimeRelevance) != 1 || filters.CrimeRelevance[0] != "high" {
		t.Errorf("crime_relevance mismatch: %v", filters.CrimeRelevance)
	}
	if len(filters.SourceNames) != 2 {
		t.Errorf("sources count=%d, want 2", len(filters.SourceNames))
	}
	if filters.FromDate == nil {
		t.Error("from_date is nil")
	}
	if filters.ToDate == nil {
		t.Error("to_date is nil")
	}
	if filters.RfpProvince != "ON" {
		t.Errorf("rfp_province=%q, want ON", filters.RfpProvince)
	}
	if len(filters.RfpSector) != 1 || filters.RfpSector[0] != "IT" {
		t.Errorf("rfp_sector mismatch: %v", filters.RfpSector)
	}
	if filters.RfpClosingAfter != "2025-06-01" {
		t.Errorf("rfp_closing_after=%q, want 2025-06-01", filters.RfpClosingAfter)
	}
	if filters.RfpBudgetMin == nil || *filters.RfpBudgetMin != 5000 {
		t.Errorf("rfp_budget_min mismatch: %v", filters.RfpBudgetMin)
	}
	if filters.RfpBudgetMax == nil || *filters.RfpBudgetMax != 100000 {
		t.Errorf("rfp_budget_max mismatch: %v", filters.RfpBudgetMax)
	}
}

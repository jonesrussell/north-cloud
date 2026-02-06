package service //nolint:testpackage // testing unexported helpers (filter, sort, paginate, infer)

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
)

// --- NormalizeSourceName ---

func TestNormalizeSourceName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple domain", "example.com", "example_com"},
		{"https prefix", "https://example.com", "example_com"},
		{"http prefix", "http://example.com", "example_com"},
		{"hyphens replaced", "my-news-site", "my_news_site"},
		{"dots and hyphens", "my-site.co.uk", "my_site_co_uk"},
		{"uppercase converted", "EXAMPLE.COM", "example_com"},
		{"mixed case with protocol", "http://EXAMPLE.COM", "example_com"},
		{"already normalized", "example_com", "example_com"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSourceName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSourceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --- GenerateIndexName ---

func TestGenerateIndexName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		sourceName string
		indexType  domain.IndexType
		expected   string
	}{
		{"raw content", "example.com", domain.IndexTypeRawContent, "example_com_raw_content"},
		{"classified content", "example.com", domain.IndexTypeClassifiedContent, "example_com_classified_content"},
		{"article", "my-site", domain.IndexTypeArticle, "my_site_articles"},
		{"page", "my-site", domain.IndexTypePage, "my_site_pages"},
		{"with protocol", "https://news.ca", domain.IndexTypeRawContent, "news_ca_raw_content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateIndexName(tt.sourceName, tt.indexType)
			if result != tt.expected {
				t.Errorf("GenerateIndexName(%q, %q) = %q, want %q",
					tt.sourceName, tt.indexType, result, tt.expected)
			}
		})
	}
}

// --- getIndexSuffix ---

func TestGetIndexSuffix(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		indexType domain.IndexType
		expected  string
	}{
		{"raw_content", domain.IndexTypeRawContent, "_raw_content"},
		{"classified_content", domain.IndexTypeClassifiedContent, "_classified_content"},
		{"article", domain.IndexTypeArticle, "_articles"},
		{"page", domain.IndexTypePage, "_pages"},
		{"unknown type", domain.IndexType("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIndexSuffix(tt.indexType)
			if result != tt.expected {
				t.Errorf("getIndexSuffix(%q) = %q, want %q", tt.indexType, result, tt.expected)
			}
		})
	}
}

// --- isValidIndexType ---

func TestIsValidIndexType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		indexType domain.IndexType
		expected  bool
	}{
		{"raw_content", domain.IndexTypeRawContent, true},
		{"classified_content", domain.IndexTypeClassifiedContent, true},
		{"article", domain.IndexTypeArticle, true},
		{"page", domain.IndexTypePage, true},
		{"unknown", domain.IndexType("unknown"), false},
		{"empty", domain.IndexType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIndexType(tt.indexType)
			if result != tt.expected {
				t.Errorf("isValidIndexType(%q) = %v, want %v", tt.indexType, result, tt.expected)
			}
		})
	}
}

// --- filterBySearch ---

func TestFilterBySearch(t *testing.T) {
	t.Helper()

	indices := []*domain.Index{
		{Name: "example_com_raw_content"},
		{Name: "news_ca_classified_content"},
		{Name: "example_com_classified_content"},
	}

	tests := []struct {
		name     string
		search   string
		expected int
	}{
		{"match multiple", "example", 2},
		{"match one", "news", 1},
		{"case insensitive", "EXAMPLE", 2},
		{"match all", "content", 3},
		{"match none", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterBySearch(indices, tt.search)
			if len(result) != tt.expected {
				t.Errorf("filterBySearch(search=%q) returned %d results, want %d",
					tt.search, len(result), tt.expected)
			}
		})
	}
}

// --- filterByHealth ---

func TestFilterByHealth(t *testing.T) {
	t.Helper()

	indices := []*domain.Index{
		{Name: "idx1", Health: "green"},
		{Name: "idx2", Health: "yellow"},
		{Name: "idx3", Health: "green"},
		{Name: "idx4", Health: "red"},
	}

	tests := []struct {
		name     string
		health   string
		expected int
	}{
		{"green", "green", 2},
		{"yellow", "yellow", 1},
		{"red", "red", 1},
		{"case insensitive", "GREEN", 2},
		{"none match", "blue", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterByHealth(indices, tt.health)
			if len(result) != tt.expected {
				t.Errorf("filterByHealth(health=%q) returned %d results, want %d",
					tt.health, len(result), tt.expected)
			}
		})
	}
}

// --- sortIndices ---

func TestSortIndices_ByName(t *testing.T) {
	t.Helper()

	indices := []*domain.Index{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}

	sortIndices(indices, "name", "asc")

	if indices[0].Name != "alpha" || indices[1].Name != "bravo" || indices[2].Name != "charlie" {
		t.Errorf("sort by name asc = [%s, %s, %s], want [alpha, bravo, charlie]",
			indices[0].Name, indices[1].Name, indices[2].Name)
	}

	sortIndices(indices, "name", "desc")

	if indices[0].Name != "charlie" {
		t.Errorf("sort by name desc first = %s, want charlie", indices[0].Name)
	}
}

func TestSortIndices_ByDocumentCount(t *testing.T) {
	t.Helper()

	indices := []*domain.Index{
		{Name: "a", DocumentCount: 100},
		{Name: "b", DocumentCount: 10},
		{Name: "c", DocumentCount: 50},
	}

	sortIndices(indices, "document_count", "asc")

	if indices[0].DocumentCount != 10 {
		t.Errorf("first by document_count asc = %d, want 10", indices[0].DocumentCount)
	}
	if indices[2].DocumentCount != 100 {
		t.Errorf("last by document_count asc = %d, want 100", indices[2].DocumentCount)
	}
}

func TestSortIndices_ByHealth(t *testing.T) {
	t.Helper()

	indices := []*domain.Index{
		{Name: "a", Health: "red"},
		{Name: "b", Health: "green"},
		{Name: "c", Health: "yellow"},
	}

	sortIndices(indices, "health", "asc")

	if indices[0].Health != "green" {
		t.Errorf("first by health asc = %s, want green", indices[0].Health)
	}
	if indices[2].Health != "red" {
		t.Errorf("last by health asc = %s, want red", indices[2].Health)
	}
}

// --- paginateIndices ---

func TestPaginateIndices(t *testing.T) {
	t.Helper()

	indices := make([]*domain.Index, 10)
	for i := range indices {
		indices[i] = &domain.Index{Name: string(rune('a' + i))}
	}

	tests := []struct {
		name     string
		offset   int
		limit    int
		expected int
	}{
		{"first page", 0, 3, 3},
		{"second page", 3, 3, 3},
		{"last partial page", 8, 3, 2},
		{"offset beyond end", 15, 3, 0},
		{"full set", 0, 10, 10},
		{"exact boundary", 0, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paginateIndices(indices, tt.offset, tt.limit)
			if len(result) != tt.expected {
				t.Errorf("paginateIndices(offset=%d, limit=%d) returned %d items, want %d",
					tt.offset, tt.limit, len(result), tt.expected)
			}
		})
	}
}

// --- inferIndexTypeAndSource ---

func TestInferIndexTypeAndSource(t *testing.T) {
	t.Helper()

	svc := &IndexService{} // No deps needed for this method

	tests := []struct {
		name           string
		indexName      string
		expectedType   domain.IndexType
		expectedSource string
	}{
		{"raw content", "example_com_raw_content", domain.IndexTypeRawContent, "example_com"},
		{"classified content", "news_ca_classified_content", domain.IndexTypeClassifiedContent, "news_ca"},
		{"articles", "my_site_articles", domain.IndexTypeArticle, "my_site"},
		{"pages", "my_site_pages", domain.IndexTypePage, "my_site"},
		{"single part", "test", "", ""},
		{"unknown suffix", "example_com_unknown", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexType, sourceName := svc.inferIndexTypeAndSource(tt.indexName)
			if indexType != tt.expectedType {
				t.Errorf("inferIndexTypeAndSource(%q) type = %q, want %q",
					tt.indexName, indexType, tt.expectedType)
			}
			if sourceName != tt.expectedSource {
				t.Errorf("inferIndexTypeAndSource(%q) source = %q, want %q",
					tt.indexName, sourceName, tt.expectedSource)
			}
		})
	}
}

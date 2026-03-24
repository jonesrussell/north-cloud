//nolint:testpackage // Testing unexported functions requires same-package access
package repository

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildListWhere_NoFilters(t *testing.T) {
	t.Helper()
	filter := ListFilter{}

	whereClause, args := buildListWhere(filter)
	assert.Empty(t, whereClause)
	assert.Empty(t, args)
}

func TestBuildListWhere_SearchFilter(t *testing.T) {
	t.Helper()
	filter := ListFilter{Search: "example"}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "name ILIKE")
	assert.Contains(t, whereClause, "url ILIKE")
	require.Len(t, args, 1)
	assert.Equal(t, "%example%", args[0])
}

func TestBuildListWhere_EnabledFilter(t *testing.T) {
	t.Helper()
	enabled := true
	filter := ListFilter{Enabled: &enabled}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "enabled = $1")
	require.Len(t, args, 1)
	assert.Equal(t, true, args[0])
}

func TestBuildListWhere_DisabledFilter(t *testing.T) {
	t.Helper()
	enabled := false
	filter := ListFilter{Enabled: &enabled}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "enabled = $1")
	require.Len(t, args, 1)
	assert.Equal(t, false, args[0])
}

func TestBuildListWhere_FeedActiveFilter(t *testing.T) {
	t.Helper()
	active := true
	filter := ListFilter{FeedActive: &active}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "feed_disabled_at IS NULL")
	assert.Empty(t, args)
}

func TestBuildListWhere_IndigenousOnlyFilter(t *testing.T) {
	t.Helper()
	filter := ListFilter{IndigenousOnly: true}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "indigenous_region IS NOT NULL")
	assert.Empty(t, args)
}

func TestBuildListWhere_CombinedFilters(t *testing.T) {
	t.Helper()
	enabled := true
	filter := ListFilter{
		Search:         "news",
		Enabled:        &enabled,
		IndigenousOnly: true,
	}

	whereClause, args := buildListWhere(filter)
	assert.Contains(t, whereClause, "ILIKE")
	assert.Contains(t, whereClause, "enabled")
	assert.Contains(t, whereClause, "indigenous_region IS NOT NULL")
	require.Len(t, args, 2) // search + enabled
}

func TestBuildListOrder_DefaultValues(t *testing.T) {
	t.Helper()
	filter := ListFilter{}

	orderClause := buildListOrder(filter)
	assert.Equal(t, " ORDER BY name ASC", orderClause)
}

func TestBuildListOrder_ValidSortBy(t *testing.T) {
	t.Helper()
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		expected  string
	}{
		{"sort by name asc", "name", "asc", " ORDER BY name ASC"},
		{"sort by url desc", "url", "desc", " ORDER BY url DESC"},
		{"sort by enabled asc", "enabled", "asc", " ORDER BY enabled ASC"},
		{"sort by created_at desc", "created_at", "desc", " ORDER BY created_at DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := ListFilter{SortBy: tt.sortBy, SortOrder: tt.sortOrder}
			assert.Equal(t, tt.expected, buildListOrder(filter))
		})
	}
}

func TestBuildListOrder_InvalidSortBy(t *testing.T) {
	t.Helper()
	filter := ListFilter{SortBy: "DROP TABLE sources;", SortOrder: "asc"}

	orderClause := buildListOrder(filter)
	assert.Equal(t, " ORDER BY name ASC", orderClause)
}

func TestBuildListOrder_InvalidSortOrder(t *testing.T) {
	t.Helper()
	filter := ListFilter{SortBy: "name", SortOrder: "invalid"}

	orderClause := buildListOrder(filter)
	assert.Equal(t, " ORDER BY name ASC", orderClause)
}

func TestDeriveClassifiedContentIndex(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal name", "Example News", "example_news_classified_content"},
		{"empty name", "", "unknown_classified_content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveClassifiedContentIndex(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMarshalExtractionProfile_Nil(t *testing.T) {
	t.Helper()
	result := marshalExtractionProfile(nil)
	assert.Nil(t, result)
}

func TestMarshalExtractionProfile_Empty(t *testing.T) {
	t.Helper()
	profile := models.ExtractionProfileJSON{}
	result := marshalExtractionProfile(&profile)
	assert.Nil(t, result)
}

func TestMarshalExtractionProfile_WithData(t *testing.T) {
	t.Helper()
	data := models.ExtractionProfileJSON(`{"key":"value"}`)
	result := marshalExtractionProfile(&data)
	assert.JSONEq(t, `{"key":"value"}`, string(result))
}

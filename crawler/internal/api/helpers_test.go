//nolint:testpackage // Testing unexported functions parseSortParams and clampLimit
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseSortParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		query         string
		allowedFields map[string]string
		defaultSortBy string
		defaultOrder  string
		wantSortBy    string
		wantSortOrder string
	}{
		{
			name:          "uses defaults when no params",
			query:         "",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "accepts valid sort field",
			query:         "?sort_by=next_run_at&sort_order=asc",
			allowedFields: map[string]string{"created_at": "created_at", "next_run_at": "next_run_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "next_run_at",
			wantSortOrder: "asc",
		},
		{
			name:          "falls back to default for invalid field",
			query:         "?sort_by=invalid_column",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "normalizes invalid sort order",
			query:         "?sort_by=created_at&sort_order=invalid",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "maps external name to internal column",
			query:         "?sort_by=source_name",
			allowedFields: map[string]string{"source_name": "COALESCE(source_name, '')"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "COALESCE(source_name, '')",
			wantSortOrder: "desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/"+tt.query, http.NoBody)

			gotSortBy, gotSortOrder := parseSortParams(c, tt.allowedFields, tt.defaultSortBy, tt.defaultOrder)

			if gotSortBy != tt.wantSortBy {
				t.Errorf("sortBy = %q, want %q", gotSortBy, tt.wantSortBy)
			}
			if gotSortOrder != tt.wantSortOrder {
				t.Errorf("sortOrder = %q, want %q", gotSortOrder, tt.wantSortOrder)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		maxLimit int
		want     int
	}{
		{"normal value", 50, 250, 50},
		{"exceeds max", 500, 250, 250},
		{"zero uses max", 0, 250, 250},
		{"negative uses max", -10, 250, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := clampLimit(tt.limit, tt.maxLimit)
			if got != tt.want {
				t.Errorf("clampLimit(%d, %d) = %d, want %d", tt.limit, tt.maxLimit, got, tt.want)
			}
		})
	}
}

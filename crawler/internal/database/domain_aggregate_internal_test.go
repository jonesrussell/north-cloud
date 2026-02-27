package database

import (
	"testing"
)

func TestNormalizeDomainSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantCol   string
		wantOrder string
	}{
		{
			name:      "valid link_count desc",
			sortBy:    "link_count",
			sortOrder: "desc",
			wantCol:   "link_count",
			wantOrder: "DESC",
		},
		{
			name:      "valid source_count asc",
			sortBy:    "source_count",
			sortOrder: "asc",
			wantCol:   "source_count",
			wantOrder: "ASC",
		},
		{
			name:      "valid domain defaults to dl.domain",
			sortBy:    "domain",
			sortOrder: "desc",
			wantCol:   "dl.domain",
			wantOrder: "DESC",
		},
		{
			name:      "valid last_seen asc",
			sortBy:    "last_seen",
			sortOrder: "asc",
			wantCol:   "last_seen",
			wantOrder: "ASC",
		},
		{
			name:      "invalid sort defaults to link_count desc",
			sortBy:    "bogus",
			sortOrder: "desc",
			wantCol:   "link_count",
			wantOrder: "DESC",
		},
		{
			name:      "invalid order defaults to link_count desc",
			sortBy:    "link_count",
			sortOrder: "bogus",
			wantCol:   "link_count",
			wantOrder: "DESC",
		},
		{
			name:      "empty sort defaults to link_count desc",
			sortBy:    "",
			sortOrder: "desc",
			wantCol:   "link_count",
			wantOrder: "DESC",
		},
		{
			name:      "empty order defaults to link_count desc",
			sortBy:    "link_count",
			sortOrder: "",
			wantCol:   "link_count",
			wantOrder: "DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotCol, gotOrder := normalizeDomainSort(tt.sortBy, tt.sortOrder)
			if gotCol != tt.wantCol {
				t.Errorf("normalizeDomainSort(%q, %q) column = %q, want %q",
					tt.sortBy, tt.sortOrder, gotCol, tt.wantCol)
			}

			if gotOrder != tt.wantOrder {
				t.Errorf("normalizeDomainSort(%q, %q) order = %q, want %q",
					tt.sortBy, tt.sortOrder, gotOrder, tt.wantOrder)
			}
		})
	}
}

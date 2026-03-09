package admin_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/admin"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

func TestFilterIndigenousSources_RegionFilter(t *testing.T) {
	t.Helper()
	canada := "canada"
	oceania := "oceania"
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "APTN", Enabled: true, IndigenousRegion: &canada},
		{ID: uuid.New(), Name: "NITV", Enabled: true, IndigenousRegion: &oceania},
		{ID: uuid.New(), Name: "CBC", Enabled: true, IndigenousRegion: nil},
	}

	result := admin.FilterIndigenousSources(allSources, "canada", 0)
	if len(result) != 1 {
		t.Fatalf("expected 1 source for canada, got %d", len(result))
	}
	if result[0].Name != "APTN" {
		t.Errorf("expected APTN, got %s", result[0].Name)
	}
}

func TestFilterIndigenousSources_AllRegions(t *testing.T) {
	t.Helper()
	canada := "canada"
	oceania := "oceania"
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "APTN", Enabled: true, IndigenousRegion: &canada},
		{ID: uuid.New(), Name: "NITV", Enabled: true, IndigenousRegion: &oceania},
		{ID: uuid.New(), Name: "CBC", Enabled: true, IndigenousRegion: nil},
	}

	result := admin.FilterIndigenousSources(allSources, "", 0)
	if len(result) != 2 {
		t.Fatalf("expected 2 indigenous sources, got %d", len(result))
	}
}

func TestFilterIndigenousSources_Limit(t *testing.T) {
	t.Helper()
	canada := "canada"
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "S1", Enabled: true, IndigenousRegion: &canada},
		{ID: uuid.New(), Name: "S2", Enabled: true, IndigenousRegion: &canada},
		{ID: uuid.New(), Name: "S3", Enabled: true, IndigenousRegion: &canada},
	}

	result := admin.FilterIndigenousSources(allSources, "", 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 sources with limit, got %d", len(result))
	}
}

func TestFilterIndigenousSources_SkipsDisabled(t *testing.T) {
	t.Helper()
	canada := "canada"
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "Disabled", Enabled: false, IndigenousRegion: &canada},
		{ID: uuid.New(), Name: "Enabled", Enabled: true, IndigenousRegion: &canada},
	}

	result := admin.FilterIndigenousSources(allSources, "", 0)
	if len(result) != 1 {
		t.Fatalf("expected 1 enabled source, got %d", len(result))
	}
	if result[0].Name != "Enabled" {
		t.Errorf("expected Enabled, got %s", result[0].Name)
	}
}

func TestFilterIndigenousSources_NoIndigenous(t *testing.T) {
	t.Helper()
	allSources := []*sources.SourceListItem{
		{ID: uuid.New(), Name: "CBC", Enabled: true, IndigenousRegion: nil},
		{ID: uuid.New(), Name: "CTV", Enabled: true, IndigenousRegion: nil},
	}

	result := admin.FilterIndigenousSources(allSources, "", 0)
	if len(result) != 0 {
		t.Fatalf("expected 0 indigenous sources, got %d", len(result))
	}
}

func TestParseBackfillLimit(t *testing.T) {
	t.Helper()
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"valid", "25", 25},
		{"negative", "-1", 0},
		{"invalid", "abc", 0},
		{"zero", "0", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			got := admin.ParseBackfillLimit(tc.in)
			if got != tc.want {
				t.Errorf("ParseBackfillLimit(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

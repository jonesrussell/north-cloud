package importer_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
)

func TestParseIndigenousSources_Valid(t *testing.T) {
	t.Helper()
	input := `[
		{
			"name": "APTN News", "homepage": "https://www.aptnnews.ca",
			"rss": "https://www.aptnnews.ca/feed/",
			"region": "canada", "country": "CA",
			"language": "en", "render_mode": "static"
		},
		{
			"name": "NITV", "homepage": "https://www.sbs.com.au/nitv/news",
			"rss": "",
			"region": "oceania", "country": "AU",
			"language": "en", "render_mode": "dynamic"
		}
	]`
	sources, err := importer.ParseIndigenousSources(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Name != "APTN News" {
		t.Errorf("expected name 'APTN News', got %q", sources[0].Name)
	}
	if sources[1].RenderMode != "dynamic" {
		t.Errorf("expected render_mode 'dynamic', got %q", sources[1].RenderMode)
	}
}

func TestParseIndigenousSources_InvalidJSON(t *testing.T) {
	t.Helper()
	input := `{not valid json`
	_, err := importer.ParseIndigenousSources(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseIndigenousSources_Empty(t *testing.T) {
	t.Helper()
	input := `[]`
	sources, err := importer.ParseIndigenousSources(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestValidateIndigenousSource(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		src      importer.IndigenousSource
		wantErr  bool
		errMatch string
	}{
		{
			name: "valid_static_source",
			src: importer.IndigenousSource{
				Name: "APTN News", Homepage: "https://www.aptnnews.ca",
				Region: "canada", RenderMode: "static",
			},
			wantErr: false,
		},
		{
			name: "valid_dynamic_source",
			src: importer.IndigenousSource{
				Name: "NITV", Homepage: "https://www.sbs.com.au/nitv",
				Region: "oceania", RenderMode: "dynamic",
			},
			wantErr: false,
		},
		{
			name: "missing_name",
			src: importer.IndigenousSource{
				Homepage: "https://example.com", Region: "canada", RenderMode: "static",
			},
			wantErr: true, errMatch: "name is required",
		},
		{
			name: "missing_homepage",
			src: importer.IndigenousSource{
				Name: "Test", Region: "canada", RenderMode: "static",
			},
			wantErr: true, errMatch: "homepage is required",
		},
		{
			name: "invalid_url_scheme",
			src: importer.IndigenousSource{
				Name: "Test", Homepage: "ftp://example.com",
				Region: "canada", RenderMode: "static",
			},
			wantErr: true, errMatch: "homepage must start with http",
		},
		{
			name: "missing_region",
			src: importer.IndigenousSource{
				Name: "Test", Homepage: "https://example.com", RenderMode: "static",
			},
			wantErr: true, errMatch: "region is required",
		},
		{
			name: "invalid_region",
			src: importer.IndigenousSource{
				Name: "Test", Homepage: "https://example.com",
				Region: "invalid_region", RenderMode: "static",
			},
			wantErr: true, errMatch: "invalid region",
		},
		{
			name: "invalid_render_mode",
			src: importer.IndigenousSource{
				Name: "Test", Homepage: "https://example.com",
				Region: "canada", RenderMode: "javascript",
			},
			wantErr: true, errMatch: "render_mode must be",
		},
		{
			name: "all_regions_valid",
			src: importer.IndigenousSource{
				Name: "Test", Homepage: "https://example.com",
				Region: "latin_america", RenderMode: "static",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errMsg := importer.ValidateIndigenousSource(tc.src)
			if tc.wantErr && errMsg == "" {
				t.Error("expected validation error, got none")
			}
			if !tc.wantErr && errMsg != "" {
				t.Errorf("unexpected validation error: %s", errMsg)
			}
			if tc.errMatch != "" && !strings.Contains(errMsg, tc.errMatch) {
				t.Errorf("expected error containing %q, got %q", tc.errMatch, errMsg)
			}
		})
	}
}

func TestIndigenousSourceToModel_Static(t *testing.T) {
	t.Helper()
	src := importer.IndigenousSource{
		Name:       "APTN News",
		Homepage:   "https://www.aptnnews.ca",
		RSS:        "https://www.aptnnews.ca/feed/",
		Region:     "canada",
		Country:    "CA",
		Language:   "en",
		RenderMode: "static",
	}

	model, err := importer.IndigenousSourceToModel(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "APTN News" {
		t.Errorf("expected name 'APTN News', got %q", model.Name)
	}
	if model.URL != "https://www.aptnnews.ca" {
		t.Errorf("expected URL 'https://www.aptnnews.ca', got %q", model.URL)
	}
	if model.RateLimit != "10s" {
		t.Errorf("expected rate limit '10s', got %q", model.RateLimit)
	}
	if model.MaxDepth != 2 {
		t.Errorf("expected max depth 2, got %d", model.MaxDepth)
	}
	if model.IngestionMode != "feed" {
		t.Errorf("expected ingestion mode 'feed', got %q", model.IngestionMode)
	}
	if model.FeedURL == nil || *model.FeedURL != "https://www.aptnnews.ca/feed/" {
		t.Error("expected FeedURL to be set")
	}
	if model.IndigenousRegion == nil || *model.IndigenousRegion != "canada" {
		t.Error("expected IndigenousRegion to be 'canada'")
	}
	if !model.Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestIndigenousSourceToModel_Dynamic(t *testing.T) {
	t.Helper()
	src := importer.IndigenousSource{
		Name:       "NITV",
		Homepage:   "https://www.sbs.com.au/nitv/news",
		Region:     "oceania",
		RenderMode: "dynamic",
	}

	model, err := importer.IndigenousSourceToModel(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.RateLimit != "12s" {
		t.Errorf("expected rate limit '12s' for dynamic, got %q", model.RateLimit)
	}
	if model.MaxDepth != 1 {
		t.Errorf("expected max depth 1 for dynamic, got %d", model.MaxDepth)
	}
	if model.RenderMode != "dynamic" {
		t.Errorf("expected render mode 'dynamic', got %q", model.RenderMode)
	}
	if model.IngestionMode != "standard" {
		t.Errorf("expected ingestion mode 'standard' when no RSS, got %q", model.IngestionMode)
	}
	if model.FeedURL != nil {
		t.Error("expected FeedURL to be nil when no RSS")
	}
}

func TestIndigenousSourceToModel_NoRSS(t *testing.T) {
	t.Helper()
	src := importer.IndigenousSource{
		Name:       "Test",
		Homepage:   "https://example.com",
		RSS:        "",
		Region:     "us",
		RenderMode: "static",
	}

	model, err := importer.IndigenousSourceToModel(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.IngestionMode != "standard" {
		t.Errorf("expected ingestion mode 'standard', got %q", model.IngestionMode)
	}
	if model.FeedURL != nil {
		t.Error("expected FeedURL to be nil")
	}
}

func TestIndigenousSourceToModel_AllRegions(t *testing.T) {
	t.Helper()
	regions := []string{"canada", "us", "latin_america", "oceania", "europe", "asia", "africa"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			src := importer.IndigenousSource{
				Name: "Test " + region, Homepage: "https://example.com",
				Region: region, RenderMode: "static",
			}
			model, err := importer.IndigenousSourceToModel(src)
			if err != nil {
				t.Fatalf("unexpected error for region %q: %v", region, err)
			}
			if model.IndigenousRegion == nil || *model.IndigenousRegion != region {
				t.Errorf("expected region %q, got %v", region, model.IndigenousRegion)
			}
		})
	}
}

package models_test

import (
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

func TestStringArray_Value(t *testing.T) {
	tests := []struct {
		name    string
		array   *models.StringArray
		wantErr bool
		want    driver.Value
	}{
		{
			name:    "nil array returns error",
			array:   nil,
			wantErr: true,
		},
		{
			name:    "empty array returns error",
			array:   &models.StringArray{},
			wantErr: true,
		},
		{
			name:  "valid array returns JSON",
			array: stringPtr(models.StringArray{"value1", "value2"}),
			want:  []byte(`["value1","value2"]`),
		},
		{
			name:  "single value array",
			array: stringPtr(models.StringArray{"single"}),
			want:  []byte(`["single"]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.array.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("StringArray.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			validateValueResult(t, got, tt.want)
		})
	}
}

func validateValueResult(t *testing.T, got, want driver.Value) {
	t.Helper()
	gotBytes, ok := got.([]byte)
	if !ok {
		t.Errorf("StringArray.Value() = %T, want []byte", got)
		return
	}
	var gotArray models.StringArray
	if unmarshalErr := json.Unmarshal(gotBytes, &gotArray); unmarshalErr != nil {
		t.Errorf("StringArray.Value() returned invalid JSON: %v", unmarshalErr)
		return
	}
	var wantArray models.StringArray
	if unmarshalErr := json.Unmarshal(want.([]byte), &wantArray); unmarshalErr != nil {
		t.Errorf("Test setup error: invalid want JSON: %v", unmarshalErr)
		return
	}
	if len(gotArray) != len(wantArray) {
		t.Errorf("StringArray.Value() length = %d, want %d", len(gotArray), len(wantArray))
		return
	}
	for i := range gotArray {
		if gotArray[i] != wantArray[i] {
			t.Errorf("StringArray.Value() [%d] = %v, want %v", i, gotArray[i], wantArray[i])
		}
	}
}

func TestStringArray_Scan(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    models.StringArray
		wantErr bool
	}{
		{
			name:  "nil value returns nil array",
			value: nil,
			want:  nil,
		},
		{
			name:    "invalid type returns nil (Scan doesn't error, just ignores)",
			value:   "not bytes",
			want:    nil, // Scan returns nil for invalid types without error
			wantErr: false,
		},
		{
			name:  "valid JSON bytes",
			value: []byte(`["value1","value2"]`),
			want:  models.StringArray{"value1", "value2"},
		},
		{
			name:  "empty JSON array",
			value: []byte(`[]`),
			want:  models.StringArray{},
		},
		{
			name:  "single value array",
			value: []byte(`["single"]`),
			want:  models.StringArray{"single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a models.StringArray
			err := a.Scan(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringArray.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(a) != len(tt.want) {
				t.Errorf("StringArray.Scan() length = %d, want %d", len(a), len(tt.want))
				return
			}
			for i := range a {
				if a[i] != tt.want[i] {
					t.Errorf("StringArray.Scan() [%d] = %v, want %v", i, a[i], tt.want[i])
				}
			}
		})
	}
}

func TestSource_Validation(t *testing.T) {
	validSource := models.Source{
		ID:   "test-id",
		Name: "Test Source",
		URL:  "https://example.com",
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{
				Title: "h1",
				Body:  ".content",
			},
		},
	}

	// Test that valid source has all required fields
	if validSource.ID == "" {
		t.Error("Source.ID should not be empty")
	}
	if validSource.Name == "" {
		t.Error("Source.Name should not be empty")
	}
	if validSource.URL == "" {
		t.Error("Source.URL should not be empty")
	}
}

func TestArticleSelectors_MergeWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    models.ArticleSelectors
		expected models.ArticleSelectors
	}{
		{
			name:  "empty selectors get all defaults",
			input: models.ArticleSelectors{},
			expected: models.ArticleSelectors{
				Container:     "article",
				Title:         "h1",
				Body:          "article > div",
				Intro:         "p.lead",
				Byline:        ".byline",
				PublishedTime: "time[datetime]",
				TimeAgo:       "time.ago",
				JSONLD:        "script[type='application/ld+json']",
				Description:   "meta[name='description']",
				Section:       ".section",
				Keywords:      "meta[name='keywords']",
				OGTitle:       "meta[property='og:title']",
				OGDescription: "meta[property='og:description']",
				OGImage:       "meta[property='og:image']",
				OGURL:         "meta[property='og:url']",
				OGSiteName:    "meta[property='og:site_name']",
				Canonical:     "link[rel='canonical']",
				Category:      ".category",
				Author:        ".author",
			},
		},
		{
			name: "partial selectors merge with defaults",
			input: models.ArticleSelectors{
				Title: "custom-title",
				Body:  "custom-body",
			},
			expected: models.ArticleSelectors{
				Container:     "article",
				Title:         "custom-title",
				Body:          "custom-body",
				Intro:         "p.lead",
				Byline:        ".byline",
				PublishedTime: "time[datetime]",
				TimeAgo:       "time.ago",
				JSONLD:        "script[type='application/ld+json']",
				Description:   "meta[name='description']",
				Section:       ".section",
				Keywords:      "meta[name='keywords']",
				OGTitle:       "meta[property='og:title']",
				OGDescription: "meta[property='og:description']",
				OGImage:       "meta[property='og:image']",
				OGURL:         "meta[property='og:url']",
				OGSiteName:    "meta[property='og:site_name']",
				Canonical:     "link[rel='canonical']",
				Category:      ".category",
				Author:        ".author",
			},
		},
		{
			name: "all fields set don't get overridden",
			input: models.ArticleSelectors{
				Container:     "custom-container",
				Title:         "custom-title",
				Body:          "custom-body",
				Intro:         "custom-intro",
				Byline:        "custom-byline",
				PublishedTime: "custom-time",
				TimeAgo:       "custom-ago",
				JSONLD:        "custom-jsonld",
				Description:   "custom-desc",
				Section:       "custom-section",
				Keywords:      "custom-keywords",
				OGTitle:       "custom-og-title",
				OGDescription: "custom-og-desc",
				OGImage:       "custom-og-image",
				OGURL:         "custom-og-url",
				OGSiteName:    "custom-og-site",
				Canonical:     "custom-canonical",
				Category:      "custom-category",
				Author:        "custom-author",
			},
			expected: models.ArticleSelectors{
				Container:     "custom-container",
				Title:         "custom-title",
				Body:          "custom-body",
				Intro:         "custom-intro",
				Byline:        "custom-byline",
				PublishedTime: "custom-time",
				TimeAgo:       "custom-ago",
				JSONLD:        "custom-jsonld",
				Description:   "custom-desc",
				Section:       "custom-section",
				Keywords:      "custom-keywords",
				OGTitle:       "custom-og-title",
				OGDescription: "custom-og-desc",
				OGImage:       "custom-og-image",
				OGURL:         "custom-og-url",
				OGSiteName:    "custom-og-site",
				Canonical:     "custom-canonical",
				Category:      "custom-category",
				Author:        "custom-author",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.MergeWithDefaults()
			if got.Container != tt.expected.Container ||
				got.Title != tt.expected.Title ||
				got.Body != tt.expected.Body ||
				got.Intro != tt.expected.Intro ||
				got.Link != tt.expected.Link ||
				got.Image != tt.expected.Image ||
				got.Byline != tt.expected.Byline ||
				got.PublishedTime != tt.expected.PublishedTime ||
				got.TimeAgo != tt.expected.TimeAgo ||
				got.Section != tt.expected.Section ||
				got.Category != tt.expected.Category ||
				got.ArticleID != tt.expected.ArticleID ||
				got.JSONLD != tt.expected.JSONLD ||
				got.Keywords != tt.expected.Keywords ||
				got.Description != tt.expected.Description ||
				got.OGTitle != tt.expected.OGTitle ||
				got.OGDescription != tt.expected.OGDescription ||
				got.OGImage != tt.expected.OGImage ||
				got.OGURL != tt.expected.OGURL ||
				got.OGType != tt.expected.OGType ||
				got.OGSiteName != tt.expected.OGSiteName ||
				got.Canonical != tt.expected.Canonical ||
				got.Author != tt.expected.Author {
				t.Errorf("ArticleSelectors.MergeWithDefaults() = %+v, want %+v", got, tt.expected)
			}
			// Compare Exclude slice separately
			if len(got.Exclude) != len(tt.expected.Exclude) {
				t.Errorf("ArticleSelectors.MergeWithDefaults() Exclude length = %d, want %d", len(got.Exclude), len(tt.expected.Exclude))
			} else {
				for i := range got.Exclude {
					if got.Exclude[i] != tt.expected.Exclude[i] {
						t.Errorf("ArticleSelectors.MergeWithDefaults() Exclude[%d] = %s, want %s", i, got.Exclude[i], tt.expected.Exclude[i])
					}
				}
			}
		})
	}
}

func TestListSelectors_MergeWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    models.ListSelectors
		expected models.ListSelectors
	}{
		{
			name:  "empty selectors get all defaults",
			input: models.ListSelectors{},
			expected: models.ListSelectors{
				Container:    ".article-list, .articles, main",
				ArticleCards: ".article-card, article, .post",
				ArticleList:  ".article-list > li, .articles > article",
			},
		},
		{
			name: "partial selectors merge with defaults",
			input: models.ListSelectors{
				Container: "custom-container",
			},
			expected: models.ListSelectors{
				Container:    "custom-container",
				ArticleCards: ".article-card, article, .post",
				ArticleList:  ".article-list > li, .articles > article",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.MergeWithDefaults()
			if got.Container != tt.expected.Container ||
				got.ArticleCards != tt.expected.ArticleCards ||
				got.ArticleList != tt.expected.ArticleList {
				t.Errorf("ListSelectors.MergeWithDefaults() = %+v, want %+v", got, tt.expected)
			}
			// Compare ExcludeFromList slice separately
			if len(got.ExcludeFromList) != len(tt.expected.ExcludeFromList) {
				t.Errorf("ListSelectors.MergeWithDefaults() ExcludeFromList length = %d, want %d", len(got.ExcludeFromList), len(tt.expected.ExcludeFromList))
			} else {
				for i := range got.ExcludeFromList {
					if got.ExcludeFromList[i] != tt.expected.ExcludeFromList[i] {
						t.Errorf("ListSelectors.MergeWithDefaults() ExcludeFromList[%d] = %s, want %s", i, got.ExcludeFromList[i], tt.expected.ExcludeFromList[i])
					}
				}
			}
		})
	}
}

func TestPageSelectors_MergeWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    models.PageSelectors
		expected models.PageSelectors
	}{
		{
			name:  "empty selectors get all defaults",
			input: models.PageSelectors{},
			expected: models.PageSelectors{
				Container:     "main, article, body",
				Title:         "h1, title",
				Content:       "main, article, .content",
				Description:   "meta[name='description']",
				Keywords:      "meta[name='keywords']",
				OGTitle:       "meta[property='og:title']",
				OGDescription: "meta[property='og:description']",
				OGImage:       "meta[property='og:image']",
				OGURL:         "meta[property='og:url']",
				Canonical:     "link[rel='canonical']",
				Exclude: []string{
					"script, style, noscript",
					".ad, .advertisement, [class*='ad']",
					".header, .footer, nav",
					"button, form",
					".sidebar, .comments",
				},
			},
		},
		{
			name: "partial selectors merge with defaults",
			input: models.PageSelectors{
				Title: "custom-title",
			},
			expected: models.PageSelectors{
				Container:     "main, article, body",
				Title:         "custom-title",
				Content:       "main, article, .content",
				Description:   "meta[name='description']",
				Keywords:      "meta[name='keywords']",
				OGTitle:       "meta[property='og:title']",
				OGDescription: "meta[property='og:description']",
				OGImage:       "meta[property='og:image']",
				OGURL:         "meta[property='og:url']",
				Canonical:     "link[rel='canonical']",
				Exclude: []string{
					"script, style, noscript",
					".ad, .advertisement, [class*='ad']",
					".header, .footer, nav",
					"button, form",
					".sidebar, .comments",
				},
			},
		},
		{
			name: "custom exclude doesn't get overridden",
			input: models.PageSelectors{
				Exclude: []string{"custom-exclude"},
			},
			expected: models.PageSelectors{
				Container:     "main, article, body",
				Title:         "h1, title",
				Content:       "main, article, .content",
				Description:   "meta[name='description']",
				Keywords:      "meta[name='keywords']",
				OGTitle:       "meta[property='og:title']",
				OGDescription: "meta[property='og:description']",
				OGImage:       "meta[property='og:image']",
				OGURL:         "meta[property='og:url']",
				Canonical:     "link[rel='canonical']",
				Exclude:       []string{"custom-exclude"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.MergeWithDefaults()
			if got.Container != tt.expected.Container ||
				got.Title != tt.expected.Title ||
				got.Content != tt.expected.Content ||
				got.Description != tt.expected.Description ||
				got.Keywords != tt.expected.Keywords ||
				got.OGTitle != tt.expected.OGTitle ||
				got.OGDescription != tt.expected.OGDescription ||
				got.OGImage != tt.expected.OGImage ||
				got.OGURL != tt.expected.OGURL ||
				got.Canonical != tt.expected.Canonical {
				t.Errorf("PageSelectors.MergeWithDefaults() = %+v, want %+v", got, tt.expected)
			}
			// Compare Exclude separately since it's a slice
			if len(got.Exclude) != len(tt.expected.Exclude) {
				t.Errorf("PageSelectors.MergeWithDefaults() Exclude length = %d, want %d", len(got.Exclude), len(tt.expected.Exclude))
			} else {
				for i := range got.Exclude {
					if got.Exclude[i] != tt.expected.Exclude[i] {
						t.Errorf("PageSelectors.MergeWithDefaults() Exclude[%d] = %s, want %s", i, got.Exclude[i], tt.expected.Exclude[i])
					}
				}
			}
		})
	}
}

func TestSelectorConfig_MergeWithDefaults(t *testing.T) {
	input := models.SelectorConfig{
		Article: models.ArticleSelectors{
			Title: "custom-title",
		},
		List: models.ListSelectors{
			Container: "custom-list-container",
		},
		Page: models.PageSelectors{
			Title: "custom-page-title",
		},
	}

	got := input.MergeWithDefaults()

	// Check that article selectors were merged
	if got.Article.Title != "custom-title" {
		t.Errorf("SelectorConfig.MergeWithDefaults() Article.Title = %s, want custom-title", got.Article.Title)
	}
	if got.Article.Container == "" {
		t.Error("SelectorConfig.MergeWithDefaults() Article.Container should have default value")
	}

	// Check that list selectors were merged
	if got.List.Container != "custom-list-container" {
		t.Errorf("SelectorConfig.MergeWithDefaults() List.Container = %s, want custom-list-container", got.List.Container)
	}
	if got.List.ArticleCards == "" {
		t.Error("SelectorConfig.MergeWithDefaults() List.ArticleCards should have default value")
	}

	// Check that page selectors were merged
	if got.Page.Title != "custom-page-title" {
		t.Errorf("SelectorConfig.MergeWithDefaults() Page.Title = %s, want custom-page-title", got.Page.Title)
	}
	if got.Page.Container == "" {
		t.Error("SelectorConfig.MergeWithDefaults() Page.Container should have default value")
	}
}

// Helper function to convert StringArray to pointer
func stringPtr(s models.StringArray) *models.StringArray {
	return &s
}

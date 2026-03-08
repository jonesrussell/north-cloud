package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

func TestLookupTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		domain       string
		wantFound    bool
		wantTemplate string
	}{
		{
			name:         "postmedia calgary",
			domain:       "calgaryherald.com",
			wantFound:    true,
			wantTemplate: "postmedia",
		},
		{
			name:         "postmedia vancouver",
			domain:       "vancouversun.com",
			wantFound:    true,
			wantTemplate: "postmedia",
		},
		{
			name:         "postmedia national",
			domain:       "nationalpost.com",
			wantFound:    true,
			wantTemplate: "postmedia",
		},
		{
			name:         "torstar",
			domain:       "thestar.com",
			wantFound:    true,
			wantTemplate: "torstar",
		},
		{
			name:      "unknown domain",
			domain:    "unknownnews.example.com",
			wantFound: false,
		},
		{
			name:      "empty domain",
			domain:    "",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl, ok := rawcontent.LookupTemplate(tc.domain)
			if ok != tc.wantFound {
				t.Fatalf("LookupTemplate(%q) found=%v, want %v", tc.domain, ok, tc.wantFound)
			}
			if tc.wantFound && tmpl.Name != tc.wantTemplate {
				t.Errorf("LookupTemplate(%q) template=%q, want %q", tc.domain, tmpl.Name, tc.wantTemplate)
			}
		})
	}
}

func TestTemplateSelectorsNotEmpty(t *testing.T) {
	t.Parallel()

	for _, tmpl := range rawcontent.TemplateRegistry {
		if tmpl.Selectors.Body == "" && tmpl.Selectors.Container == "" {
			t.Errorf("template %q has neither Body nor Container selector", tmpl.Name)
		}
	}
}

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

func TestLookupTemplateByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		hint         string
		wantFound    bool
		wantTemplate string
	}{
		{name: "postmedia by name", hint: "postmedia", wantFound: true, wantTemplate: "postmedia"},
		{name: "wordpress by name", hint: "wordpress", wantFound: true, wantTemplate: "wordpress"},
		{name: "drupal by name", hint: "drupal", wantFound: true, wantTemplate: "drupal"},
		{name: "village_media by name", hint: "village_media", wantFound: true, wantTemplate: "village_media"},
		{name: "unknown hint", hint: "unknown_cms", wantFound: false},
		{name: "empty hint", hint: "", wantFound: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl, ok := rawcontent.LookupTemplateByName(tc.hint)
			if ok != tc.wantFound {
				t.Fatalf("LookupTemplateByName(%q) found=%v, want %v", tc.hint, ok, tc.wantFound)
			}
			if tc.wantFound && tmpl.Name != tc.wantTemplate {
				t.Errorf("LookupTemplateByName(%q) template=%q, want %q", tc.hint, tmpl.Name, tc.wantTemplate)
			}
		})
	}
}

func TestDetectTemplateByHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		html         string
		wantFound    bool
		wantTemplate string
	}{
		{
			name:         "wordpress generator meta",
			html:         `<meta name="generator" content="WordPress 6.4">`,
			wantFound:    true,
			wantTemplate: "wordpress",
		},
		{
			name:         "drupal generator meta",
			html:         `<meta name="generator" content="Drupal 10">`,
			wantFound:    true,
			wantTemplate: "drupal",
		},
		{
			name: "generic og article with article tag",
			html: `<meta property="og:type" content="article">
<article class="post">content</article>`,
			wantFound:    true,
			wantTemplate: "generic_og_article",
		},
		{
			name:      "og article without article tag",
			html:      `<meta property="og:type" content="article"><div class="post">content</div>`,
			wantFound: false,
		},
		{
			name:      "no signals",
			html:      `<html><body><p>hello</p></body></html>`,
			wantFound: false,
		},
		{
			name:      "empty html",
			html:      "",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl, ok := rawcontent.DetectTemplateByHTML(tc.html)
			if ok != tc.wantFound {
				t.Fatalf("DetectTemplateByHTML() found=%v, want %v (html: %q)", ok, tc.wantFound, tc.html)
			}
			if tc.wantFound && tmpl.Name != tc.wantTemplate {
				t.Errorf("DetectTemplateByHTML() template=%q, want %q", tmpl.Name, tc.wantTemplate)
			}
		})
	}
}

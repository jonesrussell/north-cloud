package rawcontent

import (
	"net/url"
	"strings"
	"sync"
)

// CMSTemplate defines known-good CSS selectors for a CMS platform.
// Detect is an optional function that identifies the CMS from raw HTML
// (e.g. generator meta tag); it is consulted when no domain match is found.
type CMSTemplate struct {
	Name      string
	Domains   []string
	Detect    func(html string) bool // optional; nil disables HTML-based detection
	Selectors SourceSelectors
}

// htmlDetectSize is the number of bytes of HTML examined for generator meta tag
// detection. The <head> section with meta tags is always within the first 4 KB.
const htmlDetectSize = 4096

// domainPreAllocFactor estimates map capacity from the template count.
const domainPreAllocFactor = 4

var templateRegistry = []CMSTemplate{
	{
		Name: "postmedia",
		Domains: []string{
			"calgaryherald.com", "vancouversun.com", "montrealgazette.com",
			"edmontonjournal.com", "ottawacitizen.com", "nationalpost.com",
			"leaderpost.com", "thestarphoenix.com", "lfpress.com",
			"windsorstar.com", "theprovince.com",
		},
		Selectors: SourceSelectors{
			Container: "article.article-content",
			Body:      ".article-content__content-group",
			Title:     "h1.article-title",
		},
	},
	{
		Name: "torstar",
		Domains: []string{
			"thestar.com",
		},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".c-article-body__content, .article-body-text",
			Title:     "h1",
		},
	},
	{
		Name: "village_media",
		Domains: []string{
			"villagemedia.ca", "baytoday.ca", "sudbury.com", "northernontario.ctvnews.ca",
		},
		Selectors: SourceSelectors{
			Container: ".article-detail",
			Body:      ".article-detail__body",
			Title:     "h1.article-detail__title",
		},
	},
	{
		Name: "black_press",
		Domains: []string{
			"blackpress.ca", "abbynews.com", "nanaimobulletin.com",
		},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".article-body-text, .article-body",
			Title:     "h1",
		},
	},
	{
		Name: "wordpress",
		Detect: func(html string) bool {
			return strings.Contains(html, `name="generator" content="WordPress`)
		},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".entry-content",
			Title:     "h1.entry-title",
		},
	},
	{
		Name: "drupal",
		Detect: func(html string) bool {
			return strings.Contains(html, `name="generator" content="Drupal`)
		},
		Selectors: SourceSelectors{
			Body:  ".field--name-body",
			Title: "h1.page-title",
		},
	},
	// generic_og_article MUST remain after wordpress and drupal in this slice.
	// WordPress and Drupal pages often emit og:type=article + <article>, so they
	// would match this entry first if it were ordered before their specific entries.
	{
		Name: "generic_og_article",
		Detect: func(html string) bool {
			lower := strings.ToLower(html)
			hasOGArticle := strings.Contains(lower, `og:type" content="article"`) ||
				strings.Contains(lower, `property="og:type" content="article"`)
			return hasOGArticle && strings.Contains(lower, "<article")
		},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".entry-content, [itemprop=articleBody]",
		},
	},
}

var (
	domainTemplateIndex map[string]*CMSTemplate
	nameTemplateIndex   map[string]*CMSTemplate
	domainIndexOnce     sync.Once
	nameIndexOnce       sync.Once
)

// buildDomainIndex populates domainTemplateIndex from templateRegistry.
func buildDomainIndex() {
	domainIndexOnce.Do(func() {
		domainTemplateIndex = make(map[string]*CMSTemplate, len(templateRegistry)*domainPreAllocFactor)
		for i := range templateRegistry {
			tmpl := &templateRegistry[i]
			for _, domain := range tmpl.Domains {
				domainTemplateIndex[domain] = tmpl
			}
		}
	})
}

// buildNameIndex populates nameTemplateIndex from templateRegistry.
func buildNameIndex() {
	nameIndexOnce.Do(func() {
		nameTemplateIndex = make(map[string]*CMSTemplate, len(templateRegistry))
		for i := range templateRegistry {
			tmpl := &templateRegistry[i]
			nameTemplateIndex[tmpl.Name] = tmpl
		}
	})
}

// lookupTemplate returns the CMS template for the given domain, if one exists.
func lookupTemplate(domain string) (*CMSTemplate, bool) {
	if domain == "" {
		return nil, false
	}
	buildDomainIndex()
	tmpl, ok := domainTemplateIndex[domain]
	return tmpl, ok
}

// lookupTemplateByName returns the CMS template with the given name, if one exists.
// This is used when a TemplateHint is set on the source config.
func lookupTemplateByName(name string) (*CMSTemplate, bool) {
	if name == "" {
		return nil, false
	}
	buildNameIndex()
	tmpl, ok := nameTemplateIndex[name]
	return tmpl, ok
}

// detectTemplateByHTML scans templates with Detect functions against the first
// htmlDetectSize bytes of HTML and returns the first match. This is the lowest-
// priority lookup, tried only after domain and name-hint lookups fail.
func detectTemplateByHTML(html string) (*CMSTemplate, bool) {
	if html == "" {
		return nil, false
	}
	snippet := html
	if len(snippet) > htmlDetectSize {
		snippet = snippet[:htmlDetectSize]
	}
	for i := range templateRegistry {
		tmpl := &templateRegistry[i]
		if tmpl.Detect != nil && tmpl.Detect(snippet) {
			return tmpl, true
		}
	}
	return nil, false
}

// extractHostFromURL parses a URL and returns the hostname with "www." stripped.
func extractHostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	return strings.TrimPrefix(host, "www.")
}

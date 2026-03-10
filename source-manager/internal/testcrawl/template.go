package testcrawl

import "strings"

// ExtractionSelectors holds CSS selectors for content extraction.
type ExtractionSelectors struct {
	Container string
	Title     string
	Body      string
}

// cmsTemplate defines known-good CSS selectors for a CMS platform.
type cmsTemplate struct {
	Name      string
	Domains   []string
	Detect    func(html string) bool
	Selectors ExtractionSelectors
}

// htmlDetectSize is the number of bytes of HTML examined for generator-meta detection.
const htmlDetectSize = 4096

// templateRegistry mirrors the crawler's CMS template registry.
var templateRegistry = []cmsTemplate{
	{
		Name: "postmedia",
		Domains: []string{
			"calgaryherald.com", "vancouversun.com", "montrealgazette.com",
			"edmontonjournal.com", "ottawacitizen.com", "nationalpost.com",
			"leaderpost.com", "thestarphoenix.com", "lfpress.com",
			"windsorstar.com", "theprovince.com",
		},
		Selectors: ExtractionSelectors{
			Container: "article.article-content",
			Body:      ".article-content__content-group",
			Title:     "h1.article-title",
		},
	},
	{
		Name:    "torstar",
		Domains: []string{"thestar.com"},
		Selectors: ExtractionSelectors{
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
		Selectors: ExtractionSelectors{
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
		Selectors: ExtractionSelectors{
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
		Selectors: ExtractionSelectors{
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
		Selectors: ExtractionSelectors{
			Body:  ".field--name-body",
			Title: "h1.page-title",
		},
	},
	// generic_og_article MUST remain after wordpress and drupal.
	{
		Name: "generic_og_article",
		Detect: func(html string) bool {
			lower := strings.ToLower(html)
			hasOGArticle := strings.Contains(lower, `og:type" content="article"`) ||
				strings.Contains(lower, `property="og:type" content="article"`)
			return hasOGArticle && strings.Contains(lower, "<article")
		},
		Selectors: ExtractionSelectors{
			Container: "article",
			Body:      ".entry-content, [itemprop=articleBody]",
		},
	},
}

// DetectTemplate returns the best CMS template match for the given hostname and raw HTML.
// Priority: domain match → HTML-based detection.
// Returns the template name and its selectors, or an empty name when none matched.
func DetectTemplate(hostname, rawHTML string) (name string, sel ExtractionSelectors) {
	// 1. Domain match.
	for i := range templateRegistry {
		tmpl := &templateRegistry[i]
		for _, d := range tmpl.Domains {
			if d == hostname {
				return tmpl.Name, tmpl.Selectors
			}
		}
	}

	// 2. HTML-based detection (first htmlDetectSize bytes).
	snippet := rawHTML
	if len(snippet) > htmlDetectSize {
		snippet = snippet[:htmlDetectSize]
	}
	for i := range templateRegistry {
		tmpl := &templateRegistry[i]
		if tmpl.Detect != nil && tmpl.Detect(snippet) {
			return tmpl.Name, tmpl.Selectors
		}
	}

	return "", ExtractionSelectors{}
}

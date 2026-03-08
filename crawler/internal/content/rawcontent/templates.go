package rawcontent

import (
	"net/url"
	"strings"
	"sync"
)

// CMSTemplate defines known-good CSS selectors for a CMS platform.
type CMSTemplate struct {
	Name      string
	Domains   []string
	Selectors SourceSelectors
}

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
}

// domainPreAllocFactor is used to estimate the map capacity from the template count.
const domainPreAllocFactor = 4

var (
	domainTemplateIndex map[string]*CMSTemplate
	domainIndexOnce     sync.Once
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

// lookupTemplate returns the CMS template for the given domain, if one exists.
func lookupTemplate(domain string) (*CMSTemplate, bool) {
	if domain == "" {
		return nil, false
	}
	buildDomainIndex()
	tmpl, ok := domainTemplateIndex[domain]
	return tmpl, ok
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

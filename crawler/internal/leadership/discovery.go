package leadership

import (
	"net/url"
	"strings"
)

// PageType indicates the type of page discovered.
type PageType string

const (
	PageTypeLeadership PageType = "leadership"
	PageTypeContact    PageType = "contact"
)

// DiscoveredPage represents a page found via URL/link-text heuristics.
type DiscoveredPage struct {
	URL      string   `json:"url"`
	PageType PageType `json:"page_type"`
}

// leadershipPathPatterns are URL path segments that indicate a leadership page.
//
//nolint:gochecknoglobals // static lookup
var leadershipPathPatterns = []string{
	"chief-and-council", "chief-council", "chiefandcouncil",
	"leadership", "leaders", "council", "chief",
	"governance", "band-council", "elected-officials",
}

// contactPathPatterns are URL path segments that indicate a contact page.
//
//nolint:gochecknoglobals // static lookup
var contactPathPatterns = []string{
	"contact", "contact-us", "contactus",
	"band-office", "bandoffice", "office",
	"location", "find-us", "get-in-touch",
}

// leadershipLinkTexts are anchor text patterns for leadership pages.
//
//nolint:gochecknoglobals // static lookup
var leadershipLinkTexts = []string{
	"chief and council", "chief & council", "leadership",
	"council members", "elected officials", "governance",
	"band council", "chief",
}

// contactLinkTexts are anchor text patterns for contact pages.
//
//nolint:gochecknoglobals // static lookup
var contactLinkTexts = []string{
	"contact", "contact us", "band office",
	"get in touch", "find us", "location",
}

// ClassifyURL checks if a URL path matches leadership or contact patterns.
// Returns empty string if no match.
func ClassifyURL(rawURL string) PageType {
	parsed, err := url.Parse(strings.ToLower(rawURL))
	if err != nil {
		return ""
	}

	path := parsed.Path

	for _, pattern := range leadershipPathPatterns {
		if strings.Contains(path, pattern) {
			return PageTypeLeadership
		}
	}

	for _, pattern := range contactPathPatterns {
		if strings.Contains(path, pattern) {
			return PageTypeContact
		}
	}

	return ""
}

// ClassifyLinkText checks if anchor text matches leadership or contact patterns.
func ClassifyLinkText(text string) PageType {
	lower := strings.ToLower(strings.TrimSpace(text))

	for _, pattern := range leadershipLinkTexts {
		if strings.Contains(lower, pattern) {
			return PageTypeLeadership
		}
	}

	for _, pattern := range contactLinkTexts {
		if strings.Contains(lower, pattern) {
			return PageTypeContact
		}
	}

	return ""
}

// DiscoverPages examines a list of link URL+text pairs and returns discovered pages.
func DiscoverPages(baseURL string, links []Link) []DiscoveredPage {
	seen := make(map[string]bool)
	var pages []DiscoveredPage

	for _, link := range links {
		absURL := resolveURL(baseURL, link.Href)
		if absURL == "" || seen[absURL] {
			continue
		}

		// Try URL path classification first
		pageType := ClassifyURL(absURL)
		if pageType == "" {
			// Fall back to link text
			pageType = ClassifyLinkText(link.Text)
		}

		if pageType != "" {
			seen[absURL] = true
			pages = append(pages, DiscoveredPage{
				URL:      absURL,
				PageType: pageType,
			})
		}
	}

	return pages
}

// Link represents an anchor tag found on a page.
type Link struct {
	Href string
	Text string
}

// resolveURL resolves a possibly-relative href against a base URL.
func resolveURL(baseURL, href string) string {
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	ref, refErr := url.Parse(href)
	if refErr != nil {
		return ""
	}

	resolved := base.ResolveReference(ref)
	// Only keep same-host links
	if resolved.Host != base.Host {
		return ""
	}

	return resolved.String()
}

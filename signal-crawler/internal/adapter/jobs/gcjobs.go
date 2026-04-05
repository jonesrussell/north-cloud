package jobs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

const defaultGCJobsURL = "https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440"

// GCJobsBoard fetches job postings from the Canadian government jobs portal.
// Requires a Renderer because the page is JS-rendered.
type GCJobsBoard struct {
	baseURL    string
	renderer   Renderer
	httpClient *http.Client
}

// NewGCJobs creates a GC Jobs board parser.
// If renderer is non-nil, it's used to render JS content; otherwise falls back to static fetch.
func NewGCJobs(baseURL string, renderer Renderer) *GCJobsBoard {
	if baseURL == "" {
		baseURL = defaultGCJobsURL
	}
	return &GCJobsBoard{
		baseURL:    baseURL,
		renderer:   renderer,
		httpClient: &http.Client{Timeout: defaultFetchTimeout},
	}
}

// Name returns the board identifier.
func (b *GCJobsBoard) Name() string { return "gcjobs" }

// Fetch retrieves and parses job postings from GC Jobs.
func (b *GCJobsBoard) Fetch(ctx context.Context) ([]Posting, error) {
	content, err := fetchHTML(ctx, b.baseURL, "gcjobs", b.renderer, b.httpClient)
	if err != nil {
		return nil, err
	}
	return parseGCJobsHTML(content, b.baseURL)
}

func parseGCJobsHTML(content, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("gcjobs: parse html: %w", err)
	}

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "article") && hasClass(n, "resultJobItem") {
			if p, ok := extractGCJobPosting(n, baseURL); ok {
				postings = append(postings, p)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return postings, nil
}

func extractGCJobPosting(article *html.Node, baseURL string) (Posting, bool) {
	p, ok := extractJobPosting(article, baseURL, "h3", "div", "department")
	if !ok {
		return Posting{}, false
	}
	p.Sector = "government"
	return p, true
}

package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	gcJobsTimeout    = 30 * time.Second
	gcJobsMaxBody    = 10 * 1024 * 1024 // 10 MB
	defaultGCJobsURL = "https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440"
)

// GCJobsBoard fetches job postings from the Canadian government jobs portal.
type GCJobsBoard struct {
	baseURL    string
	httpClient *http.Client
}

// NewGCJobs creates a GC Jobs board parser.
func NewGCJobs(baseURL string) *GCJobsBoard {
	if baseURL == "" {
		baseURL = defaultGCJobsURL
	}
	return &GCJobsBoard{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: gcJobsTimeout},
	}
}

// Name returns the board identifier.
func (b *GCJobsBoard) Name() string { return "gcjobs" }

// Fetch retrieves and parses job postings from GC Jobs HTML.
func (b *GCJobsBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("gcjobs: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcjobs: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("gcjobs: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, gcJobsMaxBody))
	if err != nil {
		return nil, fmt.Errorf("gcjobs: read body: %w", err)
	}

	return parseGCJobsHTML(string(body), b.baseURL)
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

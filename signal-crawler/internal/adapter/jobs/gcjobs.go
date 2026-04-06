package jobs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// defaultGCJobsURL is the base search page which returns all public jobs
// as server-rendered HTML (no JS required, no session cookie needed).
const defaultGCJobsURL = "https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440"

// GCJobsBoard fetches job postings from the Canadian government jobs portal.
// The base page renders server-side HTML; no renderer is needed.
type GCJobsBoard struct {
	baseURL    string
	httpClient *http.Client
}

// NewGCJobs creates a GC Jobs board parser.
// The renderer parameter is accepted for interface compatibility but ignored,
// since the base page is server-rendered HTML.
func NewGCJobs(baseURL string, _ Renderer) *GCJobsBoard {
	if baseURL == "" {
		baseURL = defaultGCJobsURL
	}
	return &GCJobsBoard{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultFetchTimeout},
	}
}

// Name returns the board identifier.
func (b *GCJobsBoard) Name() string { return "gcjobs" }

// Fetch retrieves and parses job postings from GC Jobs via static HTTP GET.
func (b *GCJobsBoard) Fetch(ctx context.Context) ([]Posting, error) {
	content, err := fetchHTML(ctx, b.baseURL, "gcjobs", nil, b.httpClient)
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
		// GC Jobs lists results as <li class="searchResult">
		if isElem(n, "li") && hasClass(n, "searchResult") {
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

func extractGCJobPosting(li *html.Node, baseURL string) (Posting, bool) {
	// Title is in <strong><a href="...?poster=ID">Title</a></strong>
	p, ok := extractJobPosting(li, baseURL, "strong", "", "")
	if !ok {
		return Posting{}, false
	}

	// Extract department: the text content of the li minus the title and known patterns.
	// Department appears as plain text after <br> tags inside nested divs.
	dept := extractGCDepartment(li)
	if dept != "" {
		p.Company = dept
	}

	p.Sector = "government"
	return p, true
}

// extractGCDepartment finds the department name by looking for text nodes
// that appear after <br> elements inside the listing.
func extractGCDepartment(li *html.Node) string {
	var texts []string
	afterBR := false

	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if isElem(n, "br") {
			afterBR = true
		}
		if n.Type == html.TextNode && afterBR {
			t := strings.TrimSpace(n.Data)
			if t != "" && !strings.HasPrefix(t, "$") && !strings.HasPrefix(t, "Closing") && t != "," {
				texts = append(texts, t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(li)

	// The first text after a <br> that's long enough is typically the department.
	for _, t := range texts {
		if len(t) > 5 {
			return t
		}
	}
	return ""
}

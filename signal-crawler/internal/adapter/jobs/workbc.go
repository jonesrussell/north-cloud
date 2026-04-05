package jobs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// Renderer renders JS-heavy pages and returns the HTML.
type Renderer interface {
	Render(ctx context.Context, url string) (string, error)
}

const defaultWorkBCURL = "https://www.workbc.ca/find-jobs/browse-jobs"

// WorkBCBoard fetches job postings from WorkBC (British Columbia).
// If a renderer is provided, it uses it for JS-rendered content;
// otherwise it falls back to static HTTP fetch.
type WorkBCBoard struct {
	baseURL    string
	renderer   Renderer
	httpClient *http.Client
}

// NewWorkBC creates a WorkBC board parser with an optional renderer.
func NewWorkBC(baseURL string, renderer Renderer) *WorkBCBoard {
	if baseURL == "" {
		baseURL = defaultWorkBCURL
	}
	return &WorkBCBoard{
		baseURL:    baseURL,
		renderer:   renderer,
		httpClient: &http.Client{Timeout: defaultFetchTimeout},
	}
}

// Name returns the board identifier.
func (b *WorkBCBoard) Name() string { return "workbc" }

// Fetch retrieves and parses job postings from WorkBC.
func (b *WorkBCBoard) Fetch(ctx context.Context) ([]Posting, error) {
	content, err := fetchHTML(ctx, b.baseURL, "workbc", b.renderer, b.httpClient)
	if err != nil {
		return nil, err
	}
	return parseWorkBCHTML(content, b.baseURL)
}

func parseWorkBCHTML(content, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("workbc: parse html: %w", err)
	}

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "div") && hasClass(n, "job-posting") {
			if p, ok := extractWorkBCPosting(n, baseURL); ok {
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

func extractWorkBCPosting(div *html.Node, baseURL string) (Posting, bool) {
	p, ok := extractJobPosting(div, baseURL, "h2", "span", "employer")
	if !ok {
		return Posting{}, false
	}
	p.Sector = "government"
	return p, true
}

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

// Renderer renders JS-heavy pages and returns the HTML.
type Renderer interface {
	Render(ctx context.Context, url string) (string, error)
}

const (
	workBCTimeout    = 30 * time.Second
	workBCMaxBody    = 10 * 1024 * 1024 // 10 MB
	defaultWorkBCURL = "https://www.workbc.ca/find-jobs/jobs"
)

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
		httpClient: &http.Client{Timeout: workBCTimeout},
	}
}

// Name returns the board identifier.
func (b *WorkBCBoard) Name() string { return "workbc" }

// Fetch retrieves and parses job postings from WorkBC.
func (b *WorkBCBoard) Fetch(ctx context.Context) ([]Posting, error) {
	content, err := b.fetchHTML(ctx)
	if err != nil {
		return nil, err
	}
	return parseWorkBCHTML(content, b.baseURL)
}

func (b *WorkBCBoard) fetchHTML(ctx context.Context) (string, error) {
	// Use renderer if available (for JS-rendered content)
	if b.renderer != nil {
		rendered, err := b.renderer.Render(ctx, b.baseURL)
		if err != nil {
			return "", fmt.Errorf("workbc: render: %w", err)
		}
		return rendered, nil
	}

	// Static fallback
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("workbc: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("workbc: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("workbc: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, workBCMaxBody))
	if err != nil {
		return "", fmt.Errorf("workbc: read body: %w", err)
	}

	return string(body), nil
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
	var p Posting
	var foundTitle bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Look for <h2><a href="/jobs/12345">Title</a></h2>
		if isElem(n, "h2") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if isElem(c, "a") {
					href := getNodeAttr(c, "href")
					p.Title = nodeText(c)
					if strings.HasPrefix(href, "/") {
						p.URL = strings.TrimRight(baseURL, "/") + href
					} else {
						p.URL = href
					}
					// Extract job ID from path like /jobs/12345
					parts := strings.Split(strings.TrimRight(href, "/"), "/")
					if len(parts) > 0 {
						p.ID = parts[len(parts)-1]
					}
					foundTitle = true
				}
			}
		}

		// Look for <span class="employer">Company</span>
		if isElem(n, "span") && hasClass(n, "employer") {
			p.Company = nodeText(n)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(div)

	if !foundTitle || p.Title == "" {
		return Posting{}, false
	}
	p.Sector = "government"
	return p, true
}

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
	wwrTimeout    = 30 * time.Second
	wwrMaxBody    = 10 * 1024 * 1024 // 10 MB
	defaultWWRURL = "https://weworkremotely.com/remote-jobs"
)

// WWRBoard fetches job postings from WeWorkRemotely by parsing HTML.
type WWRBoard struct {
	baseURL    string
	httpClient *http.Client
}

// NewWWR creates a WeWorkRemotely board parser.
func NewWWR(baseURL string) *WWRBoard {
	if baseURL == "" {
		baseURL = defaultWWRURL
	}
	return &WWRBoard{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: wwrTimeout},
	}
}

// Name returns the board identifier.
func (b *WWRBoard) Name() string { return "wwr" }

// Fetch retrieves and parses job postings from WeWorkRemotely HTML.
func (b *WWRBoard) Fetch(ctx context.Context) ([]Posting, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("wwr: create request: %w", err)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wwr: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wwr: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, wwrMaxBody))
	if err != nil {
		return nil, fmt.Errorf("wwr: read body: %w", err)
	}

	return parseWWRHTML(string(body), b.baseURL)
}

func parseWWRHTML(content, baseURL string) ([]Posting, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("wwr: parse html: %w", err)
	}

	var postings []Posting
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Look for <li> elements containing job links
		if isElem(n, "li") {
			if p, ok := extractWWRPosting(n, baseURL); ok {
				postings = append(postings, p)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return postings, nil
}

func extractWWRPosting(li *html.Node, baseURL string) (Posting, bool) {
	// Find the <a> element with an href containing /remote-jobs/
	var p Posting
	var found bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, "a") {
			if ok := extractWWRLink(n, baseURL, &p); ok {
				found = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(li)

	if !found || p.Title == "" {
		return Posting{}, false
	}
	p.Sector = "tech"
	return p, true
}

// extractWWRLink parses a single <a> element for a WWR job link and populates the posting.
func extractWWRLink(n *html.Node, baseURL string, p *Posting) bool {
	href := getNodeAttr(n, "href")
	if !strings.Contains(href, "/remote-jobs/") {
		return false
	}

	if strings.HasPrefix(href, "/") {
		p.URL = strings.TrimRight(baseURL, "/") + href
	} else {
		p.URL = href
	}

	parts := strings.Split(strings.TrimRight(href, "/"), "/")
	if len(parts) > 0 {
		p.ID = parts[len(parts)-1]
	}

	extractWWRSpans(n, p)
	return true
}

// extractWWRSpans reads company and title spans from inside a WWR job link.
func extractWWRSpans(link *html.Node, p *Posting) {
	for c := link.FirstChild; c != nil; c = c.NextSibling {
		if !isElem(c, "span") {
			continue
		}
		cls := getNodeAttr(c, "class")
		text := nodeText(c)
		if strings.Contains(cls, "company") {
			p.Company = text
		} else if strings.Contains(cls, "title") {
			p.Title = text
		}
	}
}

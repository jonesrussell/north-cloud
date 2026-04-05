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
			href := getNodeAttr(n, "href")
			if strings.Contains(href, "/remote-jobs/") {
				found = true
				if strings.HasPrefix(href, "/") {
					p.URL = strings.TrimRight(baseURL, "/") + href
				} else {
					p.URL = href
				}
				// Extract slug as ID
				parts := strings.Split(strings.TrimRight(href, "/"), "/")
				if len(parts) > 0 {
					p.ID = parts[len(parts)-1]
				}
				// Look for company and title spans inside <a>
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if isElem(c, "span") {
						cls := getNodeAttr(c, "class")
						text := nodeText(c)
						if strings.Contains(cls, "company") {
							p.Company = text
						} else if strings.Contains(cls, "title") {
							p.Title = text
						}
					}
				}
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

// HTML parsing helpers (package-private).

func isElem(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func getNodeAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

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
	defaultFetchTimeout = 60 * time.Second
	defaultMaxBody      = 10 * 1024 * 1024 // 10 MB
)

// fetchHTML fetches a page using the renderer if available, falling back to static HTTP GET.
func fetchHTML(ctx context.Context, targetURL, boardName string, renderer Renderer, httpClient *http.Client) (string, error) {
	if renderer != nil {
		content, err := renderer.Render(ctx, targetURL)
		if err != nil {
			return "", fmt.Errorf("%s: renderer: %w", boardName, err)
		}
		return content, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("%s: create request: %w", boardName, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: fetch: %w", boardName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%s: HTTP %d", boardName, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBody))
	if err != nil {
		return "", fmt.Errorf("%s: read body: %w", boardName, err)
	}

	return string(body), nil
}

// HTML parsing helpers shared across board parsers.

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

func hasClass(n *html.Node, cls string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == cls {
					return true
				}
			}
		}
	}
	return false
}

// extractJobPosting extracts a posting from an HTML node by looking for a heading
// with a link (title + URL) and a company element matched by tag and class.
func extractJobPosting(root *html.Node, baseURL, headingTag, companyTag, companyClass string) (Posting, bool) {
	var p Posting
	var foundTitle bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElem(n, headingTag) {
			if fillPostingFromHeading(n, baseURL, &p) {
				foundTitle = true
			}
		}

		if isElem(n, companyTag) && hasClass(n, companyClass) {
			p.Company = nodeText(n)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	if !foundTitle || p.Title == "" {
		return Posting{}, false
	}
	return p, true
}

// fillPostingFromHeading extracts title, URL, and ID from a heading's child link.
func fillPostingFromHeading(heading *html.Node, baseURL string, p *Posting) bool {
	link := findChildLink(heading)
	if link == nil {
		return false
	}

	href := getNodeAttr(link, "href")
	p.Title = nodeText(link)
	if strings.HasPrefix(href, "/") {
		p.URL = strings.TrimRight(baseURL, "/") + href
	} else {
		p.URL = href
	}

	parts := strings.Split(strings.TrimRight(href, "/"), "/")
	if len(parts) > 0 {
		p.ID = parts[len(parts)-1]
	}
	return true
}

// findChildLink returns the first <a> child of a node, or nil.
func findChildLink(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isElem(c, "a") {
			return c
		}
	}
	return nil
}

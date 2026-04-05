package jobs

import (
	"strings"

	"golang.org/x/net/html"
)

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

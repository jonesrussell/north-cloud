package api

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// skipElements are tags whose text content should not be included.
//
//exhaustive:ignore
var skipElements = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Nav:      true,
	atom.Head:     true,
	atom.Header:   true,
	atom.Footer:   true,
	atom.Aside:    true,
	atom.Noscript: true,
	atom.Svg:      true,
}

// ExtractTextFromHTML strips HTML tags and returns visible text content.
// It removes script, style, nav, header, footer, and other non-content elements.
func ExtractTextFromHTML(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}

	var b strings.Builder
	extractNode(&b, doc)

	return collapseWhitespace(b.String())
}

// extractNode recursively walks the HTML tree, collecting text from visible nodes.
func extractNode(b *strings.Builder, n *html.Node) {
	if n.Type == html.ElementNode && skipElements[n.DataAtom] {
		return
	}

	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		b.WriteByte(' ')
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractNode(b, c)
	}
}

const growDivisor = 2

// collapseWhitespace normalizes whitespace: trims, collapses runs of spaces/newlines.
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s) / growDivisor)
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// CountWords returns the number of whitespace-delimited words in the text.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

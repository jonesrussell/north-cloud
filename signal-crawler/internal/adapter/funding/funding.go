package funding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"golang.org/x/net/html"
)

const defaultHTTPTimeout = 30 * time.Second

// Adapter scrapes government grant portal HTML pages for funding signals.
type Adapter struct {
	urls       []string
	httpClient *http.Client
}

// New creates a new funding Adapter that will scrape the given URLs.
func New(urls []string) *Adapter {
	return &Adapter{
		urls:       urls,
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// Name returns the short identifier for this adapter.
func (a *Adapter) Name() string {
	return "funding"
}

// Scan fetches each configured URL, parses grant rows, and returns signals.
// Continues on per-URL errors, returning partial results with a combined error.
func (a *Adapter) Scan(ctx context.Context) ([]adapter.Signal, error) {
	var allSignals []adapter.Signal
	var errs []error

	for _, rawURL := range a.urls {
		grants, err := a.fetchAndParse(ctx, rawURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("funding adapter: fetch %s: %w", rawURL, err))
			continue
		}
		allSignals = append(allSignals, grants...)
	}

	return allSignals, errors.Join(errs...)
}

type grantRow struct {
	org     string
	link    string
	program string
	amount  string
	orgType string
}

func (a *Adapter) fetchAndParse(ctx context.Context, rawURL string) ([]adapter.Signal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("funding adapter: HTTP %d fetching %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	rows, err := parseGrantRows(string(body))
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	var signals []adapter.Signal
	for _, row := range rows {
		sourceURL := row.link
		if sourceURL != "" && !strings.HasPrefix(sourceURL, "http") {
			ref, err := url.Parse(row.link)
			if err == nil {
				sourceURL = base.ResolveReference(ref).String()
			}
		}

		sig := adapter.Signal{
			Label:            fmt.Sprintf("%s — %s", row.org, row.program),
			ExternalID:       url.QueryEscape(row.org) + "|" + url.QueryEscape(row.program),
			SourceURL:        sourceURL,
			SignalStrength:   70,
			FundingStatus:    "awarded",
			OrganizationType: row.orgType,
			Notes:            fmt.Sprintf("Received %s. Likely needs tech implementation.", row.amount),
		}
		signals = append(signals, sig)
	}

	return signals, nil
}

// parseGrantRows parses the HTML and extracts grant rows from div.views-row elements.
func parseGrantRows(htmlContent string) ([]grantRow, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var rows []grantRow
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElement(n, "div") && hasClass(n, "views-row") {
			row := extractRow(n)
			if row.org != "" {
				rows = append(rows, row)
			}
			return // don't recurse into rows we've already handled
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return rows, nil
}

func extractRow(n *html.Node) grantRow {
	var row grantRow
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if isElement(n, "span") {
			cls := getAttr(n, "class")
			switch {
			case strings.Contains(cls, "views-field-title"):
				// org name is text of the <a> child; link is href
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if isElement(c, "a") {
						row.org = strings.TrimSpace(textContent(c))
						row.link = getAttr(c, "href")
					}
				}
			case strings.Contains(cls, "views-field-field-grant-program"):
				row.program = strings.TrimSpace(textContent(n))
			case strings.Contains(cls, "views-field-field-amount"):
				row.amount = strings.TrimSpace(textContent(n))
			case strings.Contains(cls, "views-field-field-org-type"):
				row.orgType = strings.TrimSpace(textContent(n))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return row
}

func isElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
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

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
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
	return sb.String()
}

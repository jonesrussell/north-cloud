// Package discovery: metadata enrichment for source candidates.

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	enrichmentTimeout     = 15 * time.Second
	enrichmentMaxBodySize = 256 * 1024 // 256KB for metadata only

	riskWeightURLSpam       = 0.5
	riskWeightAdultContent  = 0.4
	riskWeightMinimalMeta   = 0.1
	riskScoreCap             = 1.0
)

// Enricher fetches and infers metadata for a candidate URL (title, favicon, robots, category, etc.).
type Enricher struct {
	httpClient *http.Client
	log        Logger
}

// NewEnricher creates a new metadata enricher.
func NewEnricher(log Logger) *Enricher {
	return &Enricher{
		httpClient: &http.Client{Timeout: enrichmentTimeout},
		log:        log,
	}
}

// Enrich fetches the candidate URL (lightweight: follow redirects, limit body), parses title/favicon,
// fetches robots.txt for the host, and infers category and template. Results are deterministic and logged.
func (e *Enricher) Enrich(ctx context.Context, canonicalURL string) (*Enrichment, error) {
	parsed, err := url.Parse(canonicalURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	baseURL := parsed.Scheme + "://" + parsed.Host

	enrichment := &Enrichment{
		FetchedAt:        time.Now().UTC(),
		EnrichmentReason: "lightweight fetch",
	}

	// Fetch page (limited size) for title and favicon
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, canonicalURL, http.NoBody)
	if reqErr != nil {
		return nil, fmt.Errorf("create request: %w", reqErr)
	}
	req.Header.Set("User-Agent", "NorthCloud-SourceDiscovery/1.0")

	resp, doErr := e.httpClient.Do(req)
	if doErr != nil {
		enrichment.EnrichmentReason = "fetch failed: " + doErr.Error()
		return enrichment, nil // return partial so pipeline can record failure
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		enrichment.EnrichmentReason = fmt.Sprintf("http status %d", resp.StatusCode)
		return enrichment, nil
	}

	// Limit body read
	body := make([]byte, 0, enrichmentMaxBodySize)
	buf := make([]byte, 4096)
	for len(body) < enrichmentMaxBodySize {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	title, favicon := parseTitleAndFavicon(string(body), baseURL)
	enrichment.Title = title
	enrichment.FaviconURL = favicon
	if title != "" || favicon != "" {
		enrichment.EnrichmentReason = "parsed title/favicon from head"
	}

	// Robots.txt pre-check for host
	robotsURL := baseURL + "/robots.txt"
	robotsReq, rErr := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, http.NoBody)
	if rErr != nil {
		enrichment.RobotsTxtFetched = false
		return enrichment, nil
	}
	robotsReq.Header.Set("User-Agent", "NorthCloud-SourceDiscovery/1.0")
	robotsResp, robotsErr := e.httpClient.Do(robotsReq)
	if robotsErr == nil {
		enrichment.RobotsTxtFetched = true
		allowed := checkRobotsAllowed(robotsResp, "NorthCloud-SourceDiscovery/1.0", "/")
		enrichment.RobotsTxtAllowed = &allowed
		robotsResp.Body.Close()
	}

	// Infer category from URL path and title (deterministic rules)
	enrichment.Category = inferCategory(canonicalURL, title)
	enrichment.TemplateHint = inferTemplateHint(parsed.Hostname(), enrichment.Category)
	enrichment.RateLimitSuggested = "10"

	e.log.Info("Metadata enrichment completed",
		"url", canonicalURL,
		"title", title,
		"category", enrichment.Category,
		"template_hint", enrichment.TemplateHint,
		"reason", enrichment.EnrichmentReason,
	)

	return enrichment, nil
}

// parseTitleAndFavicon extracts <title> and first favicon link from HTML head.
func parseTitleAndFavicon(htmlBody, baseURL string) (title, favicon string) {
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		return "", ""
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
			if n.Data == "link" {
				for _, a := range n.Attr {
					if a.Key == "rel" && strings.ToLower(a.Val) == "icon" {
						for _, b := range n.Attr {
							if b.Key == "href" {
								favicon = resolveURL(baseURL, b.Val)
								return
							}
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title, favicon
}

func resolveURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := baseURL.Parse(ref)
	if err != nil {
		return ref
	}
	return refURL.String()
}

// checkRobotsAllowed does a simple check: if we find "User-agent: *" and "Disallow: /" then not allowed.
// Full robots.txt parsing would use a library; this is a minimal deterministic check.
func checkRobotsAllowed(resp *http.Response, userAgent, path string) bool {
	if resp.StatusCode != http.StatusOK {
		return true // no robots.txt or error -> allow
	}
	// Minimal: read body and check for Disallow: /
	body := make([]byte, 2048)
	n, _ := resp.Body.Read(body)
	text := strings.ToLower(string(body[:n]))
	// If there's a disallow for / we consider it not allowed for generic crawler
	if strings.Contains(text, "disallow: /") {
		return false
	}
	return true
}

// inferCategory returns a category from URL and title (deterministic).
func inferCategory(rawURL, title string) string {
	lower := strings.ToLower(rawURL + " " + title)
	if strings.Contains(lower, "/news/") || strings.Contains(lower, "news") {
		return "news"
	}
	if strings.Contains(lower, "/blog/") || strings.Contains(lower, "blog") {
		return "blog"
	}
	if strings.Contains(lower, "shop") || strings.Contains(lower, "store") || strings.Contains(lower, "commerce") {
		return "commerce"
	}
	return "blog"
}

// inferTemplateHint returns a template hint from host and category.
func inferTemplateHint(host, category string) string {
	host = strings.ToLower(host)
	if strings.Contains(host, "substack") {
		return "substack"
	}
	if strings.Contains(host, "medium.com") {
		return "medium"
	}
	if strings.Contains(host, "wordpress") {
		return "wordpress"
	}
	return ""
}

var riskSpamPattern = regexp.MustCompile(`(casino|viagra|lottery|click-here)`)

// RiskScore computes a simple risk score (0-1) from rules; higher = riskier.
// Used for spam/adult/low-value; reasons are appended to riskReasons.
func RiskScore(canonicalURL string, enrichment *Enrichment) (score float64, reasons []string) {
	reasons = make([]string, 0, 4)
	score = 0

	lower := strings.ToLower(canonicalURL)
	if riskSpamPattern.MatchString(lower) {
		score += riskWeightURLSpam
		reasons = append(reasons, "url_spam_indicator")
	}
	if enrichment != nil && enrichment.AdultContent {
		score += riskWeightAdultContent
		reasons = append(reasons, "adult_content")
	}
	if enrichment != nil && enrichment.Title == "" && enrichment.FaviconURL == "" {
		score += riskWeightMinimalMeta
		reasons = append(reasons, "minimal_metadata")
	}
	if score > riskScoreCap {
		score = riskScoreCap
	}
	return score, reasons
}

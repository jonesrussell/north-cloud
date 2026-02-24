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
	enrichmentReadBufSize = 4096       // read buffer for body fetch
	robotsTxtMaxBodySize  = 2048       // max bytes to read from robots.txt

	riskWeightURLSpam      = 0.5
	riskWeightAdultContent = 0.4
	riskWeightMinimalMeta  = 0.1
	riskScoreCap           = 1.0

	defaultRateLimitPerMinute = "10"

	riskFactorCount = 4 // max number of risk factors (pre-allocate reasons slice)
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
// fetches robots.txt for the host, and infers category and template.
// Rule-based inference (category, template) is deterministic; network-dependent fields reflect state at fetch time.
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

	body := make([]byte, 0, enrichmentMaxBodySize)
	buf := make([]byte, enrichmentReadBufSize)
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
	if robotsErr != nil {
		e.log.Info("Robots.txt fetch failed, treating as unfetched",
			"url", robotsURL,
			"error", robotsErr.Error(),
		)
	} else {
		enrichment.RobotsTxtFetched = true
		allowed := checkRobotsAllowed(robotsResp)
		enrichment.RobotsTxtAllowed = &allowed
		robotsResp.Body.Close()
	}

	// Infer category from URL path and title (deterministic rules)
	enrichment.Category = inferCategory(canonicalURL, title)
	enrichment.TemplateHint = inferTemplateHint(parsed.Hostname())
	enrichment.RateLimitSuggested = defaultRateLimitPerMinute

	e.log.Info("Metadata enrichment completed",
		"url", canonicalURL,
		"title", title,
		"category", enrichment.Category,
		"template_hint", enrichment.TemplateHint,
		"reason", enrichment.EnrichmentReason,
	)

	return enrichment, nil
}

// parseTitleAndFavicon extracts the first <title> element and first <link rel="icon"> href
// from the HTML document. Searches the entire document tree (not just <head>).
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
			if n.Data == "link" && favicon == "" {
				if href := extractFaviconHref(n); href != "" {
					favicon = resolveURL(baseURL, href)
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

// extractFaviconHref returns the href from a <link rel="icon"> element, or "".
func extractFaviconHref(n *html.Node) string {
	isIcon := false
	href := ""
	for _, a := range n.Attr {
		if a.Key == "rel" && strings.EqualFold(a.Val, "icon") {
			isIcon = true
		}
		if a.Key == "href" {
			href = a.Val
		}
	}
	if isIcon {
		return href
	}
	return ""
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

// checkRobotsAllowed performs a conservative check: returns false if the robots.txt body
// contains a "disallow: /" line (block-all), regardless of which User-agent block it appears under.
// A proper robots.txt parser should replace this for production use.
func checkRobotsAllowed(resp *http.Response) bool {
	if resp.StatusCode != http.StatusOK {
		return true // no robots.txt or error -> allow
	}
	body := make([]byte, robotsTxtMaxBodySize)
	n, readErr := resp.Body.Read(body)
	if n == 0 && readErr != nil {
		return true // could not read body -> allow (conservative)
	}
	text := strings.ToLower(string(body[:n]))
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "disallow: /" {
			return false
		}
	}
	return true
}

var (
	categoryNewsPattern     = regexp.MustCompile(`\bnews\b`)
	categoryBlogPattern     = regexp.MustCompile(`\bblog\b`)
	categoryCommercePattern = regexp.MustCompile(`\b(shop|store|commerce)\b`)
)

// inferCategory returns a category by keyword matching against the URL path and title.
// URL checks use path-segment matching; title checks use word-boundary matching.
// Falls back to "blog" when no keywords match.
func inferCategory(rawURL, title string) string {
	lowerURL := strings.ToLower(rawURL)
	lowerTitle := strings.ToLower(title)
	if strings.Contains(lowerURL, "/news") || categoryNewsPattern.MatchString(lowerTitle) {
		return "news"
	}
	if strings.Contains(lowerURL, "/blog") || categoryBlogPattern.MatchString(lowerTitle) {
		return "blog"
	}
	if strings.Contains(lowerURL, "/shop") || strings.Contains(lowerURL, "/store") || categoryCommercePattern.MatchString(lowerTitle) {
		return "commerce"
	}
	return "blog"
}

// inferTemplateHint returns a template hint based on the hostname.
// Known platforms: substack, medium, wordpress.
func inferTemplateHint(host string) string {
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

// RiskScore computes a risk score (0.0 to 1.0) from deterministic rules.
// Returns the score and a list of triggered risk reasons.
// Current factors: URL spam patterns (0.5), adult content flag (0.4), missing metadata (0.1).
// Score is capped at 1.0.
func RiskScore(canonicalURL string, enrichment *Enrichment) (score float64, reasons []string) {
	reasons = make([]string, 0, riskFactorCount)
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

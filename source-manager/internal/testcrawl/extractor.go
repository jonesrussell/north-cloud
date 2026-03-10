package testcrawl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// rawTextPreviewLen is the number of characters to include in the raw_text_preview field.
const rawTextPreviewLen = 500

// textDensityMinChars is the minimum character count for a candidate density element.
const textDensityMinChars = 200

// densityNoiseFragments are class/id substrings indicating non-content elements.
var densityNoiseFragments = []string{
	"nav", "menu", "sidebar", "header", "footer", "ad-", "banner",
	"promo", "comment", "social", "related", "widget",
}

// fallbackContainerSelectors are tried in order when no source/template selector matches.
var fallbackContainerSelectors = []string{
	"article",
	"main",
	".content",
	".post-content",
	".entry-content",
	"[role='main']",
	"[role='article']",
}

// ExtractionResult holds the outcome of the real extraction pipeline.
type ExtractionResult struct {
	PageType         string `json:"page_type"`
	TemplateDetected string `json:"template_detected"`
	ExtractionMethod string `json:"extraction_method"`
	Title            string `json:"title"`
	WordCount        int    `json:"word_count"`
	RawTextPreview   string `json:"raw_text_preview"`
	SelectorsTried   string `json:"selectors_tried"`
	SelectorMatched  string `json:"selector_matched"`
}

// Extractor runs the real extraction pipeline on a URL.
// It is intentionally kept separate from the metadata.Extractor to avoid
// mixing the two concern areas (selector preview vs metadata prefill).
type Extractor struct {
	logger      infralogger.Logger
	httpFetcher *httpFetcher
}

// NewExtractor creates a new testcrawl Extractor with an SSRF-safe HTTP client.
func NewExtractor(log infralogger.Logger) *Extractor {
	return &Extractor{
		logger:      log,
		httpFetcher: newHTTPFetcher(),
	}
}

// Extract runs the full extraction pipeline for the given URL and optional CSS selectors.
// sourceTitle and sourceBody are the selectors configured on the source (may be empty).
func (e *Extractor) Extract(
	ctx context.Context,
	rawURL string,
	sourceTitle, sourceBody, sourceContainer string,
) (*ExtractionResult, error) {
	// 1. URL pre-filter.
	filterResult := FilterURL(rawURL)
	if !filterResult.Allowed {
		return nil, fmt.Errorf("URL rejected by pre-filter: %s", filterResult.Reason)
	}

	// 2. Validate URL scheme and block dangerous hosts (reuses metadata package helper).
	if err := validateURL(rawURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// 3. Fetch the page.
	body, err := e.httpFetcher.Fetch(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	e.logger.Info("Fetched URL for test-crawl extraction",
		infralogger.String("url", rawURL),
		infralogger.Int("body_bytes", len(body)),
	)

	// 4. Parse HTML.
	doc, parseErr := goquery.NewDocumentFromReader(strings.NewReader(body))
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", parseErr)
	}

	// 5. Detect CMS template.
	hostname := extractHostname(rawURL)
	templateName, templateSel := DetectTemplate(hostname, body)

	// 6. Determine selectors to try (priority: source → template → heuristic).
	selectorsTried, matchedSelector, rawText := runExtractionPipeline(
		doc, sourceBody, sourceContainer, templateSel,
	)

	// 7. Extract title.
	title := extractTitle(doc, sourceTitle, templateSel.Title)

	// 8. Page type classification.
	wordCount := len(strings.Fields(rawText))
	linkCount := doc.Find("a").Length()
	ogType, _ := doc.Find("meta[property='og:type']").Attr("content")
	jsonLDType := extractJSONLDType(doc)
	articleTagCount, hasDateTime, hasSignInText := extractHTMLSignals(body)

	pageType := ClassifyPageType(pageTypeSignals{
		title:           title,
		wordCount:       wordCount,
		linkCount:       linkCount,
		ogType:          ogType,
		jsonLDType:      jsonLDType,
		articleTagCount: articleTagCount,
		hasDateTime:     hasDateTime,
		hasSignInText:   hasSignInText,
	})

	// 9. Build preview.
	preview := rawText
	if len(preview) > rawTextPreviewLen {
		preview = preview[:rawTextPreviewLen]
	}

	return &ExtractionResult{
		PageType:         pageType,
		TemplateDetected: templateName,
		ExtractionMethod: matchedSelector,
		Title:            title,
		WordCount:        wordCount,
		RawTextPreview:   preview,
		SelectorsTried:   selectorsTried,
		SelectorMatched:  matchedSelector,
	}, nil
}

// runExtractionPipeline tries selectors in priority order and returns:
// selectorsTried (comma-separated), matchedSelector (label), rawText.
func runExtractionPipeline(
	doc *goquery.Document,
	sourceBody, sourceContainer string,
	templateSel ExtractionSelectors,
) (selectorsTried, matchedSelector, rawText string) {
	tried := make([]string, 0, pipelineStageCount)

	// Stage 1: source-configured selector.
	if sourceContainer != "" || sourceBody != "" {
		sel := sourceContainer
		if sel == "" {
			sel = sourceBody
		}
		tried = append(tried, "source:"+sel)
		if text := extractTextFromSelector(doc, sel); text != "" {
			return strings.Join(tried, ", "), "selector", text
		}
	}

	// Stage 2: CMS template selector.
	if templateSel.Container != "" || templateSel.Body != "" {
		sel := templateSel.Container
		if sel == "" {
			sel = templateSel.Body
		}
		tried = append(tried, "template:"+sel)
		if text := extractTextFromSelector(doc, sel); text != "" {
			return strings.Join(tried, ", "), "template", text
		}
	}

	// Stage 3: heuristic fallback selectors.
	for _, sel := range fallbackContainerSelectors {
		tried = append(tried, "heuristic:"+sel)
		if text := extractTextFromSelector(doc, sel); len(strings.Fields(text)) > heuristicMinWords {
			return strings.Join(tried, ", "), "heuristic", text
		}
	}

	// Stage 4: text density.
	tried = append(tried, "density")
	if text := extractByTextDensity(doc); text != "" {
		return strings.Join(tried, ", "), "density", text
	}

	// Stage 5: body paragraphs last resort.
	tried = append(tried, "body_paragraphs")
	text := extractFromBodyParagraphs(doc)
	return strings.Join(tried, ", "), "body_paragraphs", text
}

// pipelineStageCount is the pre-allocated capacity for the tried-selectors slice.
const pipelineStageCount = 12

// heuristicMinWords is the minimum word count to accept a heuristic selector match.
const heuristicMinWords = 10

// extractTextFromSelector extracts plain text from the first element matching sel.
func extractTextFromSelector(doc *goquery.Document, sel string) string {
	if sel == "" {
		return ""
	}
	node := doc.Find(sel).First()
	if node.Length() == 0 {
		return ""
	}
	// Remove noise children.
	node.Find("script, style, nav, header, footer").Remove()
	return strings.TrimSpace(node.Text())
}

// extractByTextDensity finds the DOM element with highest content-to-link-text ratio.
func extractByTextDensity(doc *goquery.Document) string {
	body := doc.Find("body")
	if body.Length() == 0 {
		return ""
	}

	var best *goquery.Selection
	var bestScore float64

	body.Find("div, section, article, main").Each(func(_ int, s *goquery.Selection) {
		if isDensityNoise(s) {
			return
		}
		totalText := strings.TrimSpace(s.Text())
		totalLen := len(totalText)
		if totalLen < textDensityMinChars {
			return
		}
		linkLen := 0
		s.Find("a").Each(func(_ int, a *goquery.Selection) {
			linkLen += len(strings.TrimSpace(a.Text()))
		})
		contentLen := totalLen - linkLen
		if contentLen <= 0 {
			return
		}
		score := float64(contentLen) * float64(contentLen) / float64(totalLen)
		if score > bestScore {
			bestScore = score
			best = s
		}
	})

	if best == nil {
		return ""
	}
	return strings.TrimSpace(best.Text())
}

// isDensityNoise returns true when an element's class or id contains a noise fragment.
func isDensityNoise(s *goquery.Selection) bool {
	class, _ := s.Attr("class")
	id, _ := s.Attr("id")
	combined := strings.ToLower(class + " " + id)
	for _, fragment := range densityNoiseFragments {
		if strings.Contains(combined, fragment) {
			return true
		}
	}
	return false
}

// minParagraphLength is the minimum character count to include a paragraph.
const minParagraphLength = 20

// extractFromBodyParagraphs extracts text from body paragraphs as a last resort.
func extractFromBodyParagraphs(doc *goquery.Document) string {
	body := doc.Find("body")
	if body.Length() == 0 {
		return ""
	}
	body.Find("header, footer, nav, aside, script, style").Remove()
	paragraphs := body.Find("p")
	if paragraphs.Length() == 0 {
		return strings.TrimSpace(body.Text())
	}
	parts := make([]string, 0, paragraphs.Length())
	paragraphs.Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) > minParagraphLength {
			parts = append(parts, text)
		}
	})
	if len(parts) == 0 {
		return strings.TrimSpace(body.Text())
	}
	return strings.Join(parts, "\n\n")
}

// extractTitle tries the source selector, then the template selector, then OG/title/h1 fallbacks.
func extractTitle(doc *goquery.Document, sourceTitleSel, templateTitleSel string) string {
	for _, sel := range []string{sourceTitleSel, templateTitleSel} {
		if sel == "" {
			continue
		}
		if text := strings.TrimSpace(doc.Find(sel).First().Text()); text != "" {
			return text
		}
	}
	// JSON-LD headline.
	var headline string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if h := extractJSONLDHeadline(s.Text()); h != "" {
			headline = h
			return false
		}
		return true
	})
	if headline != "" {
		return headline
	}
	if og, exists := doc.Find("meta[property='og:title']").Attr("content"); exists && og != "" {
		return strings.TrimSpace(og)
	}
	if title := strings.TrimSpace(doc.Find("title").First().Text()); title != "" {
		return title
	}
	return strings.TrimSpace(doc.Find("h1").First().Text())
}

// extractJSONLDType returns the @type value from the first JSON-LD script tag, or "".
func extractJSONLDType(doc *goquery.Document) string {
	var result string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		t := extractJSONLDTypeFromText(s.Text())
		if t != "" {
			result = t
			return false
		}
		return true
	})
	return result
}

// extractHostname parses a URL and returns the hostname without "www.".
func extractHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// validateURL performs scheme and blocked-hostname validation before fetching.
// Reuses the same rules as metadata.Extractor by delegating to the same
// infrastructure (net/http + private IP guard at dial time), but scheme
// checks are duplicated here to avoid importing unexported helpers.
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("invalid URL scheme: only http and https are allowed")
	}
	hostname := strings.ToLower(parsed.Hostname())
	blockedHostnames := []string{
		"localhost",
		"metadata.google.internal",
		"169.254.169.254",
	}
	for _, blocked := range blockedHostnames {
		if hostname == blocked {
			return fmt.Errorf("blocked hostname: %s", hostname)
		}
	}
	return nil
}

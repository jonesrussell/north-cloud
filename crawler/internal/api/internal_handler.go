package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Fetch timeout constraints.
const (
	defaultFetchTimeout = 15 * time.Second
	maxFetchTimeout     = 30 * time.Second
	maxResponseBodySize = 10 * 1024 * 1024 // 10 MiB
	maxRedirects        = 10
)

// fetchRequest is the JSON body for POST /api/internal/v1/fetch.
type fetchRequest struct {
	URL     string `binding:"required,url" json:"url"`
	Timeout int    `json:"timeout"` // seconds; 0 = default (15s), capped at 30s
}

// ogMetadata holds Open Graph metadata extracted from the page.
type ogMetadata struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

// fetchResponse is the JSON response for POST /api/internal/v1/fetch.
type fetchResponse struct {
	URL         string      `json:"url"`
	FinalURL    string      `json:"final_url"`
	StatusCode  int         `json:"status_code"`
	ContentType string      `json:"content_type"`
	HTML        string      `json:"html"`
	Title       string      `json:"title"`
	Body        string      `json:"body"`
	Author      string      `json:"author"`
	Description string      `json:"description"`
	OG          *ogMetadata `json:"og"`
	DurationMS  int64       `json:"duration_ms"`
}

// InternalHandler handles internal service-to-service API requests.
type InternalHandler struct {
	logger infralogger.Logger
	client *http.Client
}

// NewInternalHandler creates a new internal handler.
func NewInternalHandler(logger infralogger.Logger) *InternalHandler {
	return &InternalHandler{
		logger: logger,
		client: &http.Client{
			// Timeout will be set per-request via context
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// Fetch handles POST /api/internal/v1/fetch.
// It fetches the given URL, extracts content from the HTML, and returns structured data.
func (h *InternalHandler) Fetch(c *gin.Context) {
	var req fetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %s", err.Error())})
		return
	}

	// Determine timeout
	timeout := defaultFetchTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
		if timeout > maxFetchTimeout {
			timeout = maxFetchTimeout
		}
	}

	start := time.Now()

	// Fetch the page
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, http.NoBody)
	if err != nil {
		h.logger.Warn("internal fetch: failed to create request",
			infralogger.String("url", req.URL),
			infralogger.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid url: %s", err.Error())})
		return
	}

	httpReq.Header.Set("User-Agent", "NorthCloud-Crawler/1.0")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		h.logger.Warn("internal fetch: request failed",
			infralogger.String("url", req.URL),
			infralogger.Error(err),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("fetch failed: %s", err.Error())})
		return
	}
	defer resp.Body.Close()

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		h.logger.Warn("internal fetch: failed to read body",
			infralogger.String("url", req.URL),
			infralogger.Error(err),
		)
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("failed to read response: %s", err.Error())})
		return
	}

	durationMS := time.Since(start).Milliseconds()

	contentType := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String()

	result := &fetchResponse{
		URL:         req.URL,
		FinalURL:    finalURL,
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		HTML:        string(body),
		DurationMS:  durationMS,
	}

	// Only extract content from HTML responses
	if isHTMLContentType(contentType) {
		extractContent(body, result)
	}

	c.JSON(http.StatusOK, result)
}

// isHTMLContentType checks if the Content-Type header indicates HTML.
func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml+xml")
}

// extractContent parses HTML and populates the fetchResponse with extracted data.
func extractContent(body []byte, result *fetchResponse) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return
	}

	result.Title = extractFetchTitle(doc)
	result.Description = extractFetchDescription(doc)
	result.Author = extractFetchAuthor(doc)
	result.Body = extractFetchBodyText(doc)
	result.OG = extractOGMetadata(doc)
}

// extractFetchTitle extracts the page title, preferring <title> then og:title.
func extractFetchTitle(doc *goquery.Document) string {
	if title := strings.TrimSpace(doc.Find("title").First().Text()); title != "" {
		return title
	}
	if ogTitle, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		return strings.TrimSpace(ogTitle)
	}
	return ""
}

// extractFetchDescription extracts the description from meta tags.
func extractFetchDescription(doc *goquery.Document) string {
	if desc, exists := doc.Find("meta[name='description']").Attr("content"); exists {
		return strings.TrimSpace(desc)
	}
	if ogDesc, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		return strings.TrimSpace(ogDesc)
	}
	return ""
}

// extractFetchAuthor extracts the author from meta tags.
func extractFetchAuthor(doc *goquery.Document) string {
	if author, exists := doc.Find("meta[name='author']").Attr("content"); exists {
		return strings.TrimSpace(author)
	}
	return ""
}

// fetchNonContentSelectors lists elements to strip before extracting body text.
const fetchNonContentSelectors = "script, style, nav, header, footer"

// extractFetchBodyText extracts the main body text from the document.
// Prefers <article> then <main> then <body>, stripping non-content elements.
func extractFetchBodyText(doc *goquery.Document) string {
	// Try <article> first
	article := doc.Find("article").First()
	if article.Length() > 0 {
		article.Find(fetchNonContentSelectors).Remove()
		return strings.TrimSpace(article.Text())
	}

	// Try <main> next
	main := doc.Find("main").First()
	if main.Length() > 0 {
		main.Find(fetchNonContentSelectors).Remove()
		return strings.TrimSpace(main.Text())
	}

	// Fall back to <body>
	body := doc.Find("body").First()
	if body.Length() > 0 {
		body.Find(fetchNonContentSelectors).Remove()
		return strings.TrimSpace(body.Text())
	}

	return ""
}

// extractOGMetadata extracts Open Graph metadata from the document.
func extractOGMetadata(doc *goquery.Document) *ogMetadata {
	og := &ogMetadata{}

	if val, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		og.Title = strings.TrimSpace(val)
	}
	if val, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		og.Description = strings.TrimSpace(val)
	}
	if val, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		og.Image = strings.TrimSpace(val)
	}
	if val, exists := doc.Find("meta[property='og:type']").Attr("content"); exists {
		og.Type = strings.TrimSpace(val)
	}
	if val, exists := doc.Find("meta[property='og:url']").Attr("content"); exists {
		og.URL = strings.TrimSpace(val)
	}
	if val, exists := doc.Find("meta[property='og:site_name']").Attr("content"); exists {
		og.SiteName = strings.TrimSpace(val)
	}

	// Return nil if no OG metadata found at all
	if og.Title == "" && og.Description == "" && og.Image == "" &&
		og.Type == "" && og.URL == "" && og.SiteName == "" {
		return nil
	}

	return og
}

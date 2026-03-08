package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// InternalExtractRequest represents a request to the internal extract endpoint.
type InternalExtractRequest struct {
	HTML       string `binding:"required" json:"html"`
	URL        string `binding:"required" json:"url"`
	SourceName string `json:"source_name"`
	Title      string `json:"title"`
}

// InternalExtractOG holds Open Graph metadata in the extract response.
type InternalExtractOG struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	URL         string `json:"url,omitempty"`
	Type        string `json:"type,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

// InternalExtractResponse represents the response from the internal extract endpoint.
type InternalExtractResponse struct {
	Title         string             `json:"title"`
	Author        string             `json:"author"`
	PublishedDate *time.Time         `json:"published_date"`
	Body          string             `json:"body"`
	WordCount     int                `json:"word_count"`
	QualityScore  int                `json:"quality_score"`
	Topics        []string           `json:"topics"`
	TopicScores   map[string]float64 `json:"topic_scores"`
	ContentType   string             `json:"content_type"`
	OG            InternalExtractOG  `json:"og"`
}

// InternalExtract handles POST /api/internal/v1/extract
// It accepts raw HTML + URL, runs the general-purpose classification pipeline,
// and returns structured JSON without domain-specific classifiers.
func (h *Handler) InternalExtract(c *gin.Context) {
	var req InternalExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid internal extract request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default source name
	sourceName := req.SourceName
	if sourceName == "" {
		sourceName = "pipelinex"
	}

	// Generate a unique ID for the content
	contentID := fmt.Sprintf("%s-%d", sourceName, time.Now().UnixNano())

	// Build a RawContent from the request
	rawContent := &domain.RawContent{
		ID:                   contentID,
		URL:                  req.URL,
		SourceName:           sourceName,
		Title:                req.Title,
		RawHTML:              req.HTML,
		CrawledAt:            time.Now(),
		ClassificationStatus: domain.StatusPending,
	}

	// Extract visible text from HTML for classification and response body
	extractedText := ExtractTextFromHTML(req.HTML)
	rawContent.RawText = extractedText
	rawContent.WordCount = CountWords(extractedText)

	h.logger.Info("Internal extract request",
		infralogger.String("content_id", contentID),
		infralogger.String("url", req.URL),
		infralogger.String("source_name", sourceName),
	)

	// Run classification
	result, err := h.classifier.Classify(c.Request.Context(), rawContent)
	if err != nil {
		h.logger.Error("Internal extract classification failed",
			infralogger.String("content_id", contentID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "classification failed"})
		return
	}

	// Determine title: prefer the request's title if provided, otherwise use whatever
	// the classifier may have detected (via rawContent.Title which may have been updated).
	title := req.Title
	if title == "" {
		title = rawContent.Title
	}

	// Build simplified response (general-purpose only)
	resp := InternalExtractResponse{
		Title:         title,
		Author:        "",
		PublishedDate: rawContent.PublishedDate,
		Body:          rawContent.RawText,
		WordCount:     rawContent.WordCount,
		QualityScore:  result.QualityScore,
		Topics:        result.Topics,
		TopicScores:   result.TopicScores,
		ContentType:   result.ContentType,
		OG: InternalExtractOG{
			Title:       rawContent.OGTitle,
			Description: rawContent.OGDescription,
			Image:       rawContent.OGImage,
			URL:         rawContent.OGURL,
			Type:        rawContent.OGType,
		},
	}

	// Ensure topics is never null in JSON
	if resp.Topics == nil {
		resp.Topics = []string{}
	}
	if resp.TopicScores == nil {
		resp.TopicScores = map[string]float64{}
	}

	h.logger.Info("Internal extract completed",
		infralogger.String("content_id", contentID),
		infralogger.String("content_type", result.ContentType),
		infralogger.Int("quality_score", result.QualityScore),
		infralogger.Int("word_count", rawContent.WordCount),
	)

	c.JSON(http.StatusOK, resp)
}

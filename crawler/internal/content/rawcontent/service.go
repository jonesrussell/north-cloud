// Package rawcontent provides a service for extracting and indexing raw content
// from any HTML page without type assumptions or validation.
package rawcontent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/metrics"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	storagepkg "github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	"github.com/jonesrussell/north-cloud/infrastructure/indigenous"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/pipeline"
)

// minPostExtractionWordCount is the minimum word count for extracted content to be indexed.
const minPostExtractionWordCount = 50

// DetectedContentTypeCtxKey is the colly request context key for detected content type
// (set by the crawler when IsStructuredContentPage returns true).
const DetectedContentTypeCtxKey = "detected_content_type"

// Interface defines the interface for processing raw content.
type Interface interface {
	// Process handles the processing of raw content from any HTML page.
	Process(e *colly.HTMLElement) error
}

// Ensure RawContentService implements Interface
var _ Interface = (*RawContentService)(nil)

// RawContentService extracts and indexes raw content from any HTML page.
// It does not perform type detection or validation - that's the classifier's job.
type RawContentService struct {
	logger                     infralogger.Logger
	storage                    types.Interface
	sources                    sources.Interface
	rawIndexer                 *storagepkg.RawContentIndexer
	pipeline                   *pipeline.Client
	recorder                   ExtractionRecorder // optional; set at crawl start for extraction quality metrics
	readabilityFallbackEnabled bool
	templateExtractions        int64 // atomic; incremented each time a CMS template provides selectors

	// Extraction quality counters (atomic).
	// pagesByType tracks indexed pages per page-type label.
	pageTypeArticle int64
	pageTypeListing int64
	pageTypeStub    int64
	pageTypeOther   int64

	// extractionByMethod tracks indexed pages per extraction-method label.
	methodSelector    int64
	methodTemplate    int64
	methodHeuristic   int64
	methodReadability int64

	// extractionSkipped tracks pages skipped before indexing per reason label.
	skipURLFilter   int64
	skipPageType    int64
	skipQualityGate int64

	// wordCountHistogram counts indexed pages per bucket.
	wordCountHistogram [metrics.WordCountBucketCount]int64
}

// NewRawContentService creates a new raw content service.
func NewRawContentService(
	log infralogger.Logger,
	storage types.Interface,
	sourcesManager sources.Interface,
	pipelineClient *pipeline.Client,
	readabilityFallbackEnabled bool,
) *RawContentService {
	rawIndexer := storagepkg.NewRawContentIndexer(storage, log)
	return &RawContentService{
		logger:                     log,
		storage:                    storage,
		sources:                    sourcesManager,
		rawIndexer:                 rawIndexer,
		pipeline:                   pipelineClient,
		readabilityFallbackEnabled: readabilityFallbackEnabled,
	}
}

// SetExtractionRecorder sets the optional recorder for extraction quality metrics.
// Called at crawl start when the job logger is available.
func (s *RawContentService) SetExtractionRecorder(r ExtractionRecorder) {
	s.recorder = r
}

// GetTemplateExtractions returns the number of pages for which a CMS template
// provided the extraction selectors during this crawl session.
// Safe to call concurrently.
func (s *RawContentService) GetTemplateExtractions() int64 {
	return atomic.LoadInt64(&s.templateExtractions)
}

// GetExtractionQualityMetrics returns a snapshot of the extraction quality counters
// accumulated since the service was started or last reset.
// Safe to call concurrently.
func (s *RawContentService) GetExtractionQualityMetrics() ExtractionQualityMetrics {
	var hist [metrics.WordCountBucketCount]int64
	for i := range s.wordCountHistogram {
		hist[i] = atomic.LoadInt64(&s.wordCountHistogram[i])
	}
	return ExtractionQualityMetrics{
		PagesByType: map[string]int64{
			pageTypeArticle: atomic.LoadInt64(&s.pageTypeArticle),
			pageTypeListing: atomic.LoadInt64(&s.pageTypeListing),
			pageTypeStub:    atomic.LoadInt64(&s.pageTypeStub),
			pageTypeOther:   atomic.LoadInt64(&s.pageTypeOther),
		},
		ExtractionByMethod: map[string]int64{
			extractionMethodSelector:    atomic.LoadInt64(&s.methodSelector),
			extractionMethodTemplate:    atomic.LoadInt64(&s.methodTemplate),
			extractionMethodHeuristic:   atomic.LoadInt64(&s.methodHeuristic),
			extractionMethodReadability: atomic.LoadInt64(&s.methodReadability),
		},
		ExtractionSkipped: map[string]int64{
			skipReasonURLFilter:   atomic.LoadInt64(&s.skipURLFilter),
			skipReasonPageType:    atomic.LoadInt64(&s.skipPageType),
			skipReasonQualityGate: atomic.LoadInt64(&s.skipQualityGate),
		},
		WordCountHistogram: hist,
	}
}

// Process implements the Interface for HTML element processing.
// Extracts raw content from any HTML page and indexes it to raw_content.
func (s *RawContentService) Process(e *colly.HTMLElement) error {
	if e == nil {
		return errors.New("HTML element is nil")
	}

	sourceURL := e.Request.URL.String()

	// Read detected content type from crawler context (set when IsStructuredContentPage returns true)
	var detectedContentType string
	if v := e.Request.Ctx.GetAny(DetectedContentTypeCtxKey); v != nil {
		if str, ok := v.(string); ok {
			detectedContentType = str
		} else {
			s.logger.Warn("detected_content_type context value is not a string",
				infralogger.String("url", sourceURL),
				infralogger.Any("value", v),
			)
		}
	}

	// Get source configuration to determine source name, selectors, and metadata.
	// Pass raw HTML for fallback template detection (WordPress/Drupal generator meta tags).
	rawHTML := string(e.Response.Body)
	sourceName, selectors, indigenousRegion, usedTemplate := s.getSourceConfig(sourceURL, rawHTML)

	// Determine extraction method for quality metrics before running extraction.
	// Priority: readability fallback > explicit selector > template > heuristic.
	// The actual readability check happens below; we refine after applyReadabilityFallbackIfNeeded.
	extractionMethod := s.resolveExtractionMethod(selectors, usedTemplate)

	// Extract raw content using generic extractor
	rawData := ExtractRawContent(
		e,
		sourceURL,
		selectors.Title,
		selectors.Body,
		selectors.Container,
		selectors.Exclude,
	)

	preReadabilityWordCount := len(strings.Fields(rawData.RawText))
	s.applyReadabilityFallbackIfNeeded(e, sourceURL, rawData)
	// If readability improved the word count past the heuristic threshold, record that method.
	if len(strings.Fields(rawData.RawText)) > preReadabilityWordCount {
		extractionMethod = extractionMethodReadability
	}

	// Validate extracted content before indexing
	if rawData.Title == "" && rawData.RawText == "" {
		atomic.AddInt64(&s.skipQualityGate, 1)
		s.logger.Debug("Skipping page with no extractable content",
			infralogger.String("url", sourceURL))
		return nil
	}

	wordCount := len(strings.Fields(rawData.RawText))
	if wordCount < minPostExtractionWordCount {
		atomic.AddInt64(&s.skipQualityGate, 1)
		s.logger.Debug("Skipping page with insufficient content",
			infralogger.String("url", sourceURL),
			infralogger.Int("word_count", wordCount),
			infralogger.Int("min_word_count", minPostExtractionWordCount))
		return nil
	}

	// Ensure raw_content index exists
	ctx := context.Background()
	if err := s.rawIndexer.EnsureRawContentIndex(ctx, sourceName); err != nil {
		s.logger.Warn("Failed to ensure raw_content index, continuing anyway",
			infralogger.Error(err),
			infralogger.String("source_name", sourceName))
	}

	// Convert RawContentData to RawContent for indexing
	rawContent := s.convertToRawContent(rawData, sourceName, detectedContentType, indigenousRegion)

	// Index to raw_content (no validation - classifier will handle that)
	err := s.rawIndexer.IndexRawContent(ctx, rawContent)
	if err != nil {
		s.logger.Error("Failed to index raw content",
			infralogger.Error(err),
			infralogger.String("url", sourceURL),
			infralogger.String("source_name", sourceName))
		return fmt.Errorf("failed to index raw content: %w", err)
	}

	// Emit pipeline event (fire-and-forget)
	s.emitIndexedEvent(ctx, sourceURL, sourceName, rawData, rawContent)

	// Record extraction quality metrics for this successfully indexed page.
	s.recordExtractionQuality(rawContent, extractionMethod)

	s.logger.Debug("Indexed raw content for classification",
		infralogger.String("url", sourceURL),
		infralogger.String("source_name", sourceName),
		infralogger.String("title", rawData.Title),
		infralogger.Int("word_count", rawContent.WordCount),
	)

	if s.recorder != nil {
		emptyTitle := rawData.Title == ""
		bodyEmpty := strings.TrimSpace(rawData.RawText) == "" || len(strings.Fields(rawData.RawText)) < 1
		s.recorder.RecordExtracted(emptyTitle, bodyEmpty)
	}

	return nil
}

// applyReadabilityFallbackIfNeeded runs readability when enabled and selector extraction yielded no or negligible content.
func (s *RawContentService) applyReadabilityFallbackIfNeeded(e *colly.HTMLElement, sourceURL string, rawData *RawContentData) {
	if !s.readabilityFallbackEnabled {
		return
	}
	needsFallback := strings.TrimSpace(rawData.RawHTML) == "" || len(strings.Fields(rawData.RawText)) < minPostExtractionWordCount
	if !needsFallback {
		return
	}
	fullHTML, err := e.DOM.Html()
	if err != nil || fullHTML == "" {
		return
	}
	rTitle, rHTML, rText := ApplyReadabilityFallback(fullHTML, sourceURL)
	if rHTML == "" && rText == "" {
		return
	}
	if rawData.Title == "" && rTitle != "" {
		rawData.Title = rTitle
	}
	if strings.TrimSpace(rawData.RawHTML) == "" && rHTML != "" {
		rawData.RawHTML = rHTML
	}
	if len(strings.Fields(rawData.RawText)) < minPostExtractionWordCount && rText != "" {
		rawData.RawText = rText
	}
}

// recordExtractionQuality updates the atomic extraction quality counters for one
// successfully indexed page. It is called after indexing succeeds so that skipped
// pages are never counted here.
func (s *RawContentService) recordExtractionQuality(rawContent *storagepkg.RawContent, method string) {
	s.recordPageType(rawContent)
	s.RecordExtractionMethod(method)
	bucketIdx := metrics.WordCountBucketIndex(rawContent.WordCount)
	atomic.AddInt64(&s.wordCountHistogram[bucketIdx], 1)
}

// recordPageType increments the atomic counter for the page type stored in Meta.
func (s *RawContentService) recordPageType(rawContent *storagepkg.RawContent) {
	pageType, _ := rawContent.Meta["page_type"].(string)
	switch pageType {
	case pageTypeArticle:
		atomic.AddInt64(&s.pageTypeArticle, 1)
	case pageTypeListing:
		atomic.AddInt64(&s.pageTypeListing, 1)
	case pageTypeStub:
		atomic.AddInt64(&s.pageTypeStub, 1)
	default:
		atomic.AddInt64(&s.pageTypeOther, 1)
	}
}

// resolveExtractionMethod determines the extraction method label based on
// available selectors and whether they came from a CMS template.
// The readability fallback is detected after extraction by comparing word
// counts, so this returns a pre-readability baseline.
func (s *RawContentService) resolveExtractionMethod(sel SourceSelectors, usedTemplate bool) string {
	hasExplicitSelector := sel.Title != "" || sel.Body != "" || sel.Container != ""
	if !hasExplicitSelector {
		// No configured selectors — extraction will use generic heuristic fallbacks.
		return extractionMethodHeuristic
	}
	if usedTemplate {
		return extractionMethodTemplate
	}
	return extractionMethodSelector
}

// RecordExtractionMethod increments the extraction method counter for the given method label.
// Valid labels: "selector", "template", "heuristic", "readability".
func (s *RawContentService) RecordExtractionMethod(method string) {
	switch method {
	case extractionMethodSelector:
		atomic.AddInt64(&s.methodSelector, 1)
	case extractionMethodTemplate:
		atomic.AddInt64(&s.methodTemplate, 1)
	case extractionMethodHeuristic:
		atomic.AddInt64(&s.methodHeuristic, 1)
	case extractionMethodReadability:
		atomic.AddInt64(&s.methodReadability, 1)
	}
}

// RecordSkip increments the skip counter for the given reason label.
// Valid labels: "url_filter", "page_type", "quality_gate".
func (s *RawContentService) RecordSkip(reason string) {
	switch reason {
	case skipReasonURLFilter:
		atomic.AddInt64(&s.skipURLFilter, 1)
	case skipReasonPageType:
		atomic.AddInt64(&s.skipPageType, 1)
	case skipReasonQualityGate:
		atomic.AddInt64(&s.skipQualityGate, 1)
	}
}

// emitIndexedEvent emits a pipeline event after successful raw content indexing.
func (s *RawContentService) emitIndexedEvent(
	ctx context.Context,
	sourceURL, sourceName string,
	rawData *RawContentData,
	rawContent *storagepkg.RawContent,
) {
	if s.pipeline == nil {
		return
	}

	indexName := sourceName + "_raw_content"
	pipelineErr := s.pipeline.Emit(ctx, pipeline.Event{
		ContentURL: sourceURL,
		SourceName: sourceName,
		Stage:      "indexed",
		OccurredAt: time.Now(),
		Metadata: map[string]any{
			"title":       rawData.Title,
			"word_count":  rawContent.WordCount,
			"index_name":  indexName,
			"document_id": rawContent.ID,
		},
	})
	if pipelineErr != nil {
		s.logger.Warn("Failed to emit pipeline event",
			infralogger.Error(pipelineErr),
			infralogger.String("url", sourceURL),
			infralogger.String("stage", "indexed"),
		)
	}
}

// getSourceConfig gets the source configuration and returns source name, selectors, indigenous region,
// and whether selectors were resolved from a CMS template (rather than explicit source config).
func (s *RawContentService) getSourceConfig(sourceURL, rawHTML string) (
	name string, sel SourceSelectors, indigenousRegion string, usedTemplate bool,
) {
	var sourceName string
	selectors := SourceSelectors{}

	if s.sources == nil {
		// No sources manager, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("No sources manager available, using URL-based source name",
			infralogger.String("source_name", sourceName),
			infralogger.String("url", sourceURL))
		return sourceName, selectors, "", false
	}

	// Try to find source by URL (matching domain)
	sourceConfig := s.findSourceByURL(sourceURL)
	if sourceConfig == nil {
		// Source not found, use URL as source name
		sourceName = extractSourceNameFromURL(sourceURL)
		s.logger.Debug("Source not found for URL, using URL-based source name",
			infralogger.String("url", sourceURL),
			infralogger.String("source_name", sourceName))
		return sourceName, selectors, "", false
	}

	// Use hostname from the URL being crawled, not the source's Name field
	// This ensures index names are based on URLs (e.g., "www.sudbury.com") rather than human-readable names
	sourceName = extractSourceNameFromURL(sourceURL)
	s.logger.Debug("Source found by URL, using URL-based source name for indexing",
		infralogger.String("url", sourceURL),
		infralogger.String("source_name", sourceName),
		infralogger.String("source_config_name", sourceConfig.Name))

	// Extract selectors from source config (if available)
	// We'll use article selectors as a guide, but won't enforce article-specific logic
	if sourceConfig.Selectors.Article.Title != "" {
		selectors.Title = sourceConfig.Selectors.Article.Title
	}
	if sourceConfig.Selectors.Article.Body != "" {
		selectors.Body = sourceConfig.Selectors.Article.Body
	}
	if sourceConfig.Selectors.Article.Container != "" {
		selectors.Container = sourceConfig.Selectors.Article.Container
	}
	if len(sourceConfig.Selectors.Article.Exclude) > 0 {
		selectors.Exclude = sourceConfig.Selectors.Article.Exclude
	}

	// If source-manager has no selectors, resolve via template registry.
	// Priority: TemplateHint (explicit) > domain lookup > HTML-based detection.
	if selectors.Title == "" && selectors.Body == "" && selectors.Container == "" {
		tmpl, tmplName := s.resolveTemplate(sourceConfig, sourceURL, rawHTML)
		if tmpl != nil {
			selectors = tmpl.Selectors
			usedTemplate = true
			atomic.AddInt64(&s.templateExtractions, 1)
			s.logger.Debug("Using CMS template selectors",
				infralogger.String("url", sourceURL),
				infralogger.String("template", tmplName))
		}
	}

	// Normalize region slug for consistency across the pipeline
	region, normalizeErr := indigenous.NormalizeRegionSlug(sourceConfig.IndigenousRegion)
	if normalizeErr != nil {
		s.logger.Warn("Invalid indigenous_region on source, ignoring",
			infralogger.Error(normalizeErr),
			infralogger.String("url", sourceURL),
			infralogger.String("raw_region", sourceConfig.IndigenousRegion))
		region = ""
	}

	return sourceName, selectors, region, usedTemplate
}

// resolveTemplate returns the best-matching CMS template for a page, along with its name.
// Lookup priority: TemplateHint (explicit override) > domain match > HTML detection.
// If TemplateHint is set but not found in the registry, a warning is logged and lookup
// falls through to domain and HTML detection. Returns nil if no template matches.
func (s *RawContentService) resolveTemplate(
	sourceConfig *sources.Config, sourceURL, rawHTML string,
) (result *CMSTemplate, resultName string) {
	// 1. TemplateHint: explicit name from source-manager config skips detection entirely.
	if sourceConfig.TemplateHint != nil {
		if found, ok := lookupTemplateByName(*sourceConfig.TemplateHint); ok {
			return found, found.Name
		}
		s.logger.Warn("TemplateHint not found in registry",
			infralogger.String("hint", *sourceConfig.TemplateHint),
			infralogger.String("url", sourceURL))
	}

	// 2. Domain match: fast, exact lookup against known CMS domains.
	hostname := extractHostFromURL(sourceURL)
	if found, ok := lookupTemplate(hostname); ok {
		return found, found.Name
	}

	// 3. HTML detection: generator meta tags and OG signals as last resort.
	if found, ok := detectTemplateByHTML(rawHTML); ok {
		return found, found.Name
	}

	return nil, ""
}

// SourceSelectors represents generic selectors for content extraction
type SourceSelectors struct {
	Title     string
	Body      string
	Container string
	Exclude   []string
}

// extractSourceNameFromURL extracts a source name from a URL
func extractSourceNameFromURL(urlStr string) string {
	// Simple extraction: use domain name
	// Example: https://example.com/article -> example_com
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")

	parts := strings.Split(urlStr, "/")
	if len(parts) > 0 {
		domainName := parts[0]
		domainName = strings.ReplaceAll(domainName, ".", "_")
		domainName = strings.ToLower(domainName)
		return domainName
	}

	return "unknown_source"
}

// findSourceByURL attempts to find a source configuration by matching the URL domain.
func (s *RawContentService) findSourceByURL(pageURL string) *sources.Config {
	if s.sources == nil {
		return nil
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil
	}

	// Get all sources and try to match by domain
	sourceConfigs, err := s.sources.GetSources()
	if err != nil {
		return nil
	}

	for i := range sourceConfigs {
		source := &sourceConfigs[i]
		// Check if domain matches any allowed domain
		for _, allowedDomain := range source.AllowedDomains {
			if allowedDomain == hostname || allowedDomain == "*."+hostname {
				return source
			}
		}
		// Also check source URL
		if sourceParsedURL, parseErr := url.Parse(source.URL); parseErr == nil {
			if sourceParsedURL.Hostname() == hostname {
				return source
			}
		}
	}

	return nil
}

// convertToRawContent converts RawContentData to storage.RawContent for indexing
func (s *RawContentService) convertToRawContent(
	rawData *RawContentData, sourceName, detectedContentType, indigenousRegion string,
) *storagepkg.RawContent {
	// Calculate word count
	wordCount := calculateWordCount(rawData.RawText)

	// Use OG type as-is from HTML extraction
	// Don't default to "article" - let classifier decide based on content characteristics
	ogType := rawData.OGType
	// Keep empty if not present in HTML

	// Build meta object with additional metadata
	meta := make(map[string]any)
	if rawData.ArticleOpinion {
		meta["article_opinion"] = rawData.ArticleOpinion
	}
	if rawData.ArticleContentTier != "" {
		meta["article_content_tier"] = rawData.ArticleContentTier
	}
	if rawData.TwitterCard != "" {
		meta["twitter_card"] = rawData.TwitterCard
	}
	if rawData.TwitterSite != "" {
		meta["twitter_site"] = rawData.TwitterSite
	}
	if rawData.OGImageWidth > 0 {
		meta["og_image_width"] = rawData.OGImageWidth
	}
	if rawData.OGImageHeight > 0 {
		meta["og_image_height"] = rawData.OGImageHeight
	}
	if rawData.OGSiteName != "" {
		meta["og_site_name"] = rawData.OGSiteName
	}
	if detectedContentType != "" {
		meta["detected_content_type"] = detectedContentType
	}
	if indigenousRegion != "" {
		meta["indigenous_region"] = indigenousRegion
	}

	// Tag page type for extraction quality measurement
	linkCount := strings.Count(rawData.RawHTML, "<a ")
	articleTagCount, hasDateTime, hasSignInText := extractHTMLSignals(rawData.RawHTML)
	pageType := classifyPageType(pageTypeSignals{
		title:               rawData.Title,
		wordCount:           wordCount,
		linkCount:           linkCount,
		ogType:              ogType,
		detectedContentType: detectedContentType,
		jsonLDType:          extractJSONLDType(rawData.JSONLDData),
		articleTagCount:     articleTagCount,
		hasDateTime:         hasDateTime,
		hasSignInText:       hasSignInText,
	})
	meta["page_type"] = pageType

	rawContent := &storagepkg.RawContent{
		ID:                   rawData.ID,
		URL:                  rawData.URL,
		SourceName:           sourceName,
		Title:                rawData.Title,
		RawText:              rawData.RawText,
		RawHTML:              rawData.RawHTML,
		MetaDescription:      rawData.MetaDescription,
		MetaKeywords:         rawData.MetaKeywords,
		OGType:               ogType,
		OGTitle:              rawData.OGTitle,
		OGDescription:        rawData.OGDescription,
		OGImage:              rawData.OGImage,
		Author:               rawData.Author,
		PublishedDate:        rawData.PublishedDate,
		CanonicalURL:         rawData.CanonicalURL,
		ArticleSection:       rawData.ArticleSection,
		JSONLDData:           rawData.JSONLDData,
		ClassificationStatus: "pending",
		CrawledAt:            time.Now(),
		WordCount:            wordCount,
	}
	// Defensive normalization so jsonld_raw never contains object/array for
	// publisher, author, image, mainEntityOfPage (avoids ES mapping conflicts).
	NormalizeJSONLDRawForIndex(rawContent.JSONLDData)

	// Add CreatedAt and UpdatedAt to meta if they exist
	if !rawData.CreatedAt.IsZero() {
		meta["created_at"] = rawData.CreatedAt
	}
	if !rawData.UpdatedAt.IsZero() {
		meta["updated_at"] = rawData.UpdatedAt
	}

	// Only add meta object if it has content
	if len(meta) > 0 {
		rawContent.Meta = meta
	}

	return rawContent
}

// calculateWordCount calculates word count from text
func calculateWordCount(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return len(words)
}

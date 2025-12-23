// Package content provides content processing types and interfaces.
package content

import (
	"context"
	"fmt"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/contenttype"
)

// ContentProcessor defines the interface for processing different types of content.
type ContentProcessor interface {
	// ContentType returns the type of content this processor can handle.
	ContentType() contenttype.Type

	// CanProcess checks if the processor can handle the given content.
	CanProcess(content contenttype.Type) bool

	// Process handles the content processing.
	Process(ctx context.Context, content any) error
}

// ProcessorRegistry manages content processors.
type ProcessorRegistry interface {
	// RegisterProcessor registers a new content processor.
	RegisterProcessor(processor ContentProcessor)

	// GetProcessor returns a processor for the given content type.
	GetProcessor(contentType contenttype.Type) (ContentProcessor, error)

	// ProcessContent processes content using the appropriate processor.
	ProcessContent(ctx context.Context, contentType contenttype.Type, content any) error
}

// Processor defines the interface for content processors.
type Processor interface {
	ContentProcessor
	ProcessorRegistry

	// Start initializes the processor.
	Start(ctx context.Context) error

	// Stop cleans up the processor.
	Stop(ctx context.Context) error
}

// ContentTypeDetector detects the type of content.
type ContentTypeDetector interface {
	// Detect detects the content type of the given content.
	Detect(content any) (contenttype.Type, error)
}

// HTMLContentTypeDetector detects content types in HTML.
type HTMLContentTypeDetector struct {
	selectors map[contenttype.Type]string
}

// NewHTMLContentTypeDetector creates a new HTML content type detector.
func NewHTMLContentTypeDetector(selectors map[contenttype.Type]string) *HTMLContentTypeDetector {
	return &HTMLContentTypeDetector{
		selectors: selectors,
	}
}

// Detect implements ContentTypeDetector.Detect.
func (d *HTMLContentTypeDetector) Detect(content any) (contenttype.Type, error) {
	e, ok := content.(*colly.HTMLElement)
	if !ok {
		return "", fmt.Errorf("invalid content type: expected *colly.HTMLElement, got %T", content)
	}

	for contentType, selector := range d.selectors {
		if e.DOM.Find(selector).Length() > 0 {
			return contentType, nil
		}
	}

	return contenttype.Page, nil
}

// ProcessingStep represents a step in a processing pipeline.
type ProcessingStep interface {
	// Process processes the content and returns the processed result.
	Process(ctx context.Context, content any) (any, error)
}

// ProcessingPipeline represents a pipeline of processing steps.
type ProcessingPipeline struct {
	steps []ProcessingStep
}

// Execute executes the pipeline on the given content.
func (p *ProcessingPipeline) Execute(ctx context.Context, content any) (any, error) {
	var err error
	for _, step := range p.steps {
		content, err = step.Process(ctx, content)
		if err != nil {
			return nil, fmt.Errorf("step failed: %w", err)
		}
	}
	return content, nil
}

// ProcessorConfig holds configuration for a processor.
type ProcessorConfig struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Options map[string]any `json:"options"`
}

// ProcessorFactory creates processors based on configuration.
type ProcessorFactory interface {
	CreateProcessor(config ProcessorConfig) (ContentProcessor, error)
}

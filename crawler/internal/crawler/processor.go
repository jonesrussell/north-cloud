// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"fmt"

	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/content/contenttype"
)

// Processor defines the interface for content processors.
type Processor interface {
	Process(ctx context.Context, content any) error
}

// ContentType represents the type of content being processed.
type ContentType string

const (
	// ContentTypeArticle represents article content.
	ContentTypeArticle ContentType = "article"
	// ContentTypeContent represents general content.
	ContentTypeContent ContentType = "content"
)

// processorMap maps content types to their processors.
type processorMap map[ContentType]Processor

// NewProcessorMap creates a new processor map.
func NewProcessorMap() processorMap {
	return make(processorMap)
}

// Add adds a processor for a specific content type.
func (m processorMap) Add(contentType ContentType, processor Processor) {
	m[contentType] = processor
}

// Get returns the processor for a specific content type.
func (m processorMap) Get(contentType ContentType) Processor {
	return m[contentType]
}

// Has returns true if a processor exists for the content type.
func (m processorMap) Has(contentType ContentType) bool {
	_, exists := m[contentType]
	return exists
}

// Processor Management Methods
// ----------------------------

// AddProcessor adds a new processor to the crawler.
func (c *Crawler) AddProcessor(processor content.Processor) {
	c.processors = append(c.processors, processor)
}

// SetArticleProcessor sets the article processor.
func (c *Crawler) SetArticleProcessor(processor content.Processor) {
	c.articleProcessor = processor
}

// SetPageProcessor sets the page processor.
func (c *Crawler) SetPageProcessor(processor content.Processor) {
	c.pageProcessor = processor
}

// GetProcessors returns the processors.
func (c *Crawler) GetProcessors() []content.Processor {
	processors := make([]content.Processor, 0, len(c.processors))
	for _, p := range c.processors {
		wrapper := &processorWrapper{
			processor: p,
			registry:  make([]content.ContentProcessor, 0),
		}
		processors = append(processors, wrapper)
	}
	return processors
}

// processorWrapper wraps a content.Processor to implement content.Processor
type processorWrapper struct {
	processor content.Processor
	registry  []content.ContentProcessor
}

// ContentType implements content.ContentProcessor
func (p *processorWrapper) ContentType() contenttype.Type {
	return p.processor.ContentType()
}

// CanProcess implements content.ContentProcessor
func (p *processorWrapper) CanProcess(ct contenttype.Type) bool {
	return p.processor.CanProcess(ct)
}

// Process implements content.ContentProcessor
func (p *processorWrapper) Process(ctx context.Context, contentData any) error {
	return p.processor.Process(ctx, contentData)
}

// RegisterProcessor implements content.ProcessorRegistry
func (p *processorWrapper) RegisterProcessor(proc content.ContentProcessor) {
	p.registry = append(p.registry, proc)
}

// GetProcessor implements content.ProcessorRegistry
func (p *processorWrapper) GetProcessor(ct contenttype.Type) (content.ContentProcessor, error) {
	for _, proc := range p.registry {
		if proc.CanProcess(ct) {
			return proc, nil
		}
	}
	return nil, fmt.Errorf("no processor found for content type: %s", ct)
}

// ProcessContent implements content.ProcessorRegistry
func (p *processorWrapper) ProcessContent(ctx context.Context, ct contenttype.Type, contentData any) error {
	proc, err := p.GetProcessor(ct)
	if err != nil {
		return err
	}
	return proc.Process(ctx, contentData)
}

// Start implements content.Processor
func (p *processorWrapper) Start(ctx context.Context) error {
	return p.processor.Start(ctx)
}

// Stop implements content.Processor
func (p *processorWrapper) Stop(ctx context.Context) error {
	return p.processor.Stop(ctx)
}

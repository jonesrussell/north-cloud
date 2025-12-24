// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/contenttype"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

// defaultProcessorsCapacity is the pre-allocated capacity for processor slice
const defaultProcessorsCapacity = 2

// HTMLProcessor processes HTML content and delegates to appropriate content processors.
type HTMLProcessor struct {
	logger       logger.Interface
	processors   []content.Processor
	unknownTypes map[contenttype.Type]int
	sources      sources.Interface
}

// NewHTMLProcessor creates a new HTMLProcessor.
func NewHTMLProcessor(log logger.Interface, sourcesManager sources.Interface) *HTMLProcessor {
	return &HTMLProcessor{
		logger:       log,
		processors:   make([]content.Processor, 0, defaultProcessorsCapacity), // Pre-allocate for article and page processors
		unknownTypes: make(map[contenttype.Type]int),
		sources:      sourcesManager,
	}
}

// CanProcess returns whether the processor can handle the given content type.
func (p *HTMLProcessor) CanProcess(contentType contenttype.Type) bool {
	return contentType == contenttype.HTML
}

// ContentType returns the content type this processor handles.
func (p *HTMLProcessor) ContentType() contenttype.Type {
	return contenttype.HTML
}

// Start initializes the processor.
func (p *HTMLProcessor) Start(ctx context.Context) error {
	return nil
}

// Stop stops the processor.
func (p *HTMLProcessor) Stop(ctx context.Context) error {
	return nil
}

// ValidateJob validates a job before processing.
func (p *HTMLProcessor) ValidateJob(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	return nil
}

// GetProcessor returns a processor for the given content type.
func (p *HTMLProcessor) GetProcessor(contentType contenttype.Type) (content.ContentProcessor, error) {
	for _, processor := range p.processors {
		if processor.CanProcess(contentType) {
			return processor, nil
		}
	}
	return nil, fmt.Errorf("no processor found for content type: %s", contentType)
}

// RegisterProcessor registers a new processor.
func (p *HTMLProcessor) RegisterProcessor(processor content.Processor) {
	p.processors = append(p.processors, processor)
}

// ProcessContent processes content using the appropriate processor.
func (p *HTMLProcessor) ProcessContent(ctx context.Context, ct contenttype.Type, contentData any) error {
	proc, err := p.GetProcessor(ct)
	if err != nil {
		return err
	}
	return proc.Process(ctx, contentData)
}

// GetUnknownTypes returns a map of content types that have no registered processor.
func (p *HTMLProcessor) GetUnknownTypes() map[contenttype.Type]int {
	return p.unknownTypes
}

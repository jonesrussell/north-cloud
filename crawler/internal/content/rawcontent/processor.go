// Package rawcontent provides a processor for extracting and indexing raw content
// from any HTML page without type assumptions.
package rawcontent

import (
	"context"
	"errors"
	"fmt"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/contenttype"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
)

// RawContentProcessor implements the content.Processor interface for raw content extraction.
// It processes any HTML page without type detection or validation.
type RawContentProcessor struct {
	logger  logger.Interface
	service Interface
}

// NewProcessor creates a new raw content processor.
func NewProcessor(
	log logger.Interface,
	service Interface,
) *RawContentProcessor {
	return &RawContentProcessor{
		logger:  log,
		service: service,
	}
}

// Process implements the content.Processor interface.
func (p *RawContentProcessor) Process(ctx context.Context, contentData any) error {
	e, ok := contentData.(*colly.HTMLElement)
	if !ok {
		return fmt.Errorf("invalid content type: expected *colly.HTMLElement, got %T", contentData)
	}

	// Use the service to process the raw content
	if err := p.service.Process(e); err != nil {
		p.logger.Error("Failed to process raw content",
			"error", err,
			"url", e.Request.URL.String())
		return fmt.Errorf("failed to process raw content: %w", err)
	}

	return nil
}

// ContentType implements the content.Processor interface.
// Returns HTML since we process any HTML page without type detection.
func (p *RawContentProcessor) ContentType() contenttype.Type {
	return contenttype.HTML
}

// CanProcess implements the content.Processor interface.
// Can process any content type since we don't distinguish between types.
func (p *RawContentProcessor) CanProcess(ct contenttype.Type) bool {
	// Can process any content type - we don't distinguish
	return true
}

// ValidateJob implements the content.Processor interface.
func (p *RawContentProcessor) ValidateJob(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	if len(job.Items) == 0 {
		return errors.New("job must have at least one item")
	}
	return nil
}

// RegisterProcessor implements content.ProcessorRegistry
func (p *RawContentProcessor) RegisterProcessor(proc content.ContentProcessor) {
	// Not implemented - we only handle raw content processing
}

// GetProcessor implements content.ProcessorRegistry
func (p *RawContentProcessor) GetProcessor(contentType contenttype.Type) (content.ContentProcessor, error) {
	// Return self for any content type
	return &rawContentContentProcessor{p}, nil
}

// rawContentContentProcessor wraps RawContentProcessor to implement content.ContentProcessor
type rawContentContentProcessor struct {
	*RawContentProcessor
}

// Process implements content.ContentProcessor
func (p *rawContentContentProcessor) Process(ctx context.Context, contentData any) error {
	return p.RawContentProcessor.Process(ctx, contentData)
}

// ContentType implements content.ContentProcessor
func (p *rawContentContentProcessor) ContentType() contenttype.Type {
	return p.RawContentProcessor.ContentType()
}

// CanProcess implements content.ContentProcessor
func (p *rawContentContentProcessor) CanProcess(ct contenttype.Type) bool {
	return p.RawContentProcessor.CanProcess(ct)
}

// ValidateJob implements content.ContentProcessor
func (p *rawContentContentProcessor) ValidateJob(job *content.Job) error {
	return p.RawContentProcessor.ValidateJob(job)
}

// Start implements content.Processor
func (p *RawContentProcessor) Start(ctx context.Context) error {
	return nil
}

// Stop implements content.Processor
func (p *RawContentProcessor) Stop(ctx context.Context) error {
	return nil
}

// ProcessContent implements content.ProcessorRegistry
func (p *RawContentProcessor) ProcessContent(ctx context.Context, ct contenttype.Type, contentData any) error {
	proc, err := p.GetProcessor(ct)
	if err != nil {
		return err
	}
	return proc.Process(ctx, contentData)
}

// Validate validates a job
func (p *RawContentProcessor) Validate(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	if len(job.Items) == 0 {
		return errors.New("job must have at least one item")
	}
	return nil
}

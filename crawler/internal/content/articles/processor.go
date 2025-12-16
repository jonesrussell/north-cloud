// Package articles provides functionality for processing and managing article content.
package articles

import (
	"context"
	"errors"
	"fmt"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/content/contenttype"
	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/processor"
	"github.com/jonesrussell/gocrawl/internal/storage/types"
)

// Type conversion helper: domain.Type and contenttype.Type are both string types,
// but we need to compare Item.Type (domain.Type) with contenttype constants.
// Since Item.Type is domain.Type, we compare with domain.TypeArticle.

// ArticleProcessor implements the common.Processor interface for articles.
type ArticleProcessor struct {
	logger         logger.Interface
	service        Interface
	validator      content.JobValidator
	storage        types.Interface
	indexName      string
	articleChannel chan *domain.Article
	articleIndexer processor.Processor
	pageIndexer    processor.Processor
}

// NewProcessor creates a new article processor.
func NewProcessor(
	log logger.Interface,
	service Interface,
	validator content.JobValidator,
	storage types.Interface,
	indexName string,
	articleChannel chan *domain.Article,
	articleIndexer processor.Processor,
	pageIndexer processor.Processor,
) *ArticleProcessor {
	return &ArticleProcessor{
		logger:         log,
		service:        service,
		validator:      validator,
		storage:        storage,
		indexName:      indexName,
		articleChannel: articleChannel,
		articleIndexer: articleIndexer,
		pageIndexer:    pageIndexer,
	}
}

// Process implements the common.Processor interface.
func (p *ArticleProcessor) Process(ctx context.Context, contentData any) error {
	e, ok := contentData.(*colly.HTMLElement)
	if !ok {
		return fmt.Errorf("invalid content type: expected *colly.HTMLElement, got %T", contentData)
	}

	// Use the service to process the article
	// The service will extract data and index it
	if err := p.service.Process(e); err != nil {
		p.logger.Error("Failed to process article",
			"error", err,
			"url", e.Request.URL.String())
		return fmt.Errorf("failed to process article: %w", err)
	}

	return nil
}

// ContentType implements the common.Processor interface.
func (p *ArticleProcessor) ContentType() contenttype.Type {
	return contenttype.Article
}

// CanProcess implements the common.Processor interface.
func (p *ArticleProcessor) CanProcess(ct contenttype.Type) bool {
	return ct == contenttype.Article
}

// ValidateJob implements the common.Processor interface.
func (p *ArticleProcessor) ValidateJob(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	if len(job.Items) == 0 {
		return errors.New("job must have at least one item")
	}
	for _, item := range job.Items {
		if item.Type != domain.TypeArticle {
			return errors.New("invalid item type: expected article")
		}
	}
	return nil
}

// RegisterProcessor implements content.ProcessorRegistry
func (p *ArticleProcessor) RegisterProcessor(proc content.ContentProcessor) {
	// Not implemented - we only handle article processing
}

// GetProcessor implements content.ProcessorRegistry
func (p *ArticleProcessor) GetProcessor(contentType contenttype.Type) (content.ContentProcessor, error) {
	if contentType == contenttype.Article {
		return &articleContentProcessor{p}, nil
	}
	return nil, errors.New("unsupported content type")
}

// articleContentProcessor wraps ArticleProcessor to implement content.ContentProcessor
type articleContentProcessor struct {
	*ArticleProcessor
}

// Process implements content.ContentProcessor
func (p *articleContentProcessor) Process(ctx context.Context, contentData any) error {
	return p.ArticleProcessor.Process(ctx, contentData)
}

// ContentType implements content.ContentProcessor
func (p *articleContentProcessor) ContentType() contenttype.Type {
	return p.ArticleProcessor.ContentType()
}

// CanProcess implements content.ContentProcessor
func (p *articleContentProcessor) CanProcess(ct contenttype.Type) bool {
	return p.ArticleProcessor.CanProcess(ct)
}

// ValidateJob implements content.ContentProcessor
func (p *articleContentProcessor) ValidateJob(job *content.Job) error {
	return p.ArticleProcessor.ValidateJob(job)
}

// Start implements content.Processor
func (p *ArticleProcessor) Start(ctx context.Context) error {
	return nil
}

// Stop implements content.Processor
func (p *ArticleProcessor) Stop(ctx context.Context) error {
	return p.Close()
}

// Close cleans up resources used by the processor.
func (p *ArticleProcessor) Close() error {
	if p.articleChannel != nil {
		close(p.articleChannel)
	}
	return nil
}

// ProcessContent implements content.ProcessorRegistry
func (p *ArticleProcessor) ProcessContent(ctx context.Context, ct contenttype.Type, contentData any) error {
	proc, err := p.GetProcessor(ct)
	if err != nil {
		return err
	}
	return proc.Process(ctx, contentData)
}

// Validate validates a job
func (p *ArticleProcessor) Validate(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	if len(job.Items) == 0 {
		return errors.New("job must have at least one item")
	}
	return nil
}

// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"errors"

	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/contenttype"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// ProcessorFactory creates content processors for different content types.
type ProcessorFactory interface {
	// CreateProcessor creates a new processor for the given content type.
	CreateProcessor(contentType contenttype.Type) (content.Processor, error)
}

// DefaultProcessorFactory implements ProcessorFactory with default processors.
type DefaultProcessorFactory struct {
	logger     logger.Interface
	storage    types.Interface
	indexName  string
	processors map[contenttype.Type]content.Processor
}

// NewProcessorFactory creates a new processor factory.
func NewProcessorFactory(
	log logger.Interface,
	storage types.Interface,
	indexName string,
) ProcessorFactory {
	return &DefaultProcessorFactory{
		logger:     log,
		storage:    storage,
		indexName:  indexName,
		processors: make(map[contenttype.Type]content.Processor),
	}
}

// CreateProcessor implements ProcessorFactory.
func (f *DefaultProcessorFactory) CreateProcessor(contentType contenttype.Type) (content.Processor, error) {
	// Check if we already have a processor for this type
	if processor, ok := f.processors[contentType]; ok {
		return processor, nil
	}

	// Create a new processor based on the content type
	switch contentType {
	case contenttype.Article:
		return nil, errors.New("article processing not implemented - use rawcontent processor instead")
	case contenttype.Page:
		return nil, errors.New("page processing not implemented - use rawcontent processor instead")
	case contenttype.Video:
		return nil, errors.New("video processing not implemented")
	case contenttype.Image:
		return nil, errors.New("image processing not implemented")
	case contenttype.HTML:
		return nil, errors.New("HTML processing not implemented")
	case contenttype.Job:
		return nil, errors.New("job processing not implemented")
	default:
		return nil, errors.New("unsupported content type")
	}
}

// Package content provides content processing types and interfaces.
package content

import (
	"context"

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

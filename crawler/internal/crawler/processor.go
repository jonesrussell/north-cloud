// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
)

// Processor Management Methods
// ----------------------------

// AddProcessor adds a new processor to the crawler.
func (c *Crawler) AddProcessor(processor content.Processor) {
	c.processors = append(c.processors, processor)
}

// GetProcessors returns the processors.
func (c *Crawler) GetProcessors() []content.Processor {
	processors := make([]content.Processor, len(c.processors))
	copy(processors, c.processors)
	return processors
}

package parser

import (
	"io"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
)

// PortalParser defines the interface for parsing procurement feed data
// from a specific portal into RFPDocuments.
type PortalParser interface {
	// Parse reads raw feed data and returns a map of docID -> RFPDocument.
	// The docID is a deterministic identifier unique to the portal.
	Parse(r io.Reader) (map[string]domain.RFPDocument, error)

	// SourceName returns the canonical source identifier (e.g., "CanadaBuys", "SEAO").
	SourceName() string
}

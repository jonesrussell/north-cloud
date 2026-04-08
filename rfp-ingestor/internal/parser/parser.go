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
	// rowErrors holds non-fatal per-row/record issues (e.g. malformed CSV rows
	// where other rows still parsed). It is nil when there are none.
	// err is set only for fatal failures (no usable result).
	Parse(r io.Reader) (docs map[string]domain.RFPDocument, rowErrors []error, err error)

	// SourceName returns the canonical source identifier (e.g., "CanadaBuys", "SEAO").
	SourceName() string
}

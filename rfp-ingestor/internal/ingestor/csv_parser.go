package ingestor

import (
	"io"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/domain"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/parser"
)

// ParseCSV delegates to the parser package for backward compatibility.
func ParseCSV(r io.Reader) ([]domain.RFPDocument, []error) {
	return parser.ParseCanadaBuysCSV(r)
}

// DocumentID delegates to the parser package for backward compatibility.
func DocumentID(doc domain.RFPDocument) string {
	return parser.CanadaBuysDocumentID(doc)
}

package contracts

import (
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// ClassifiedContentMapping returns the canonical classified_content index
// mapping as a contract Mapping. Services that write to or read from
// *_classified_content indexes should test against this mapping.
func ClassifiedContentMapping() Mapping {
	full := mappings.GetClassifiedContentMapping(1, 1)
	return extractProperties(full)
}

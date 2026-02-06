package contracts

import (
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// RawContentMapping returns the canonical raw_content index mapping as a
// contract Mapping. Services that write to or read from *_raw_content indexes
// should test against this mapping to ensure field compatibility.
func RawContentMapping() Mapping {
	full := mappings.GetRawContentMapping()
	return extractProperties(full)
}

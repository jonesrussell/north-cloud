package contracts

import (
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// RawContentMapping returns the canonical raw_content index mapping as a
// contract Mapping. Services that write to or read from *_raw_content indexes
// should test against this mapping to ensure field compatibility.
func RawContentMapping() Mapping {
	full := mappings.GetRawContentMapping(1, 0)
	return extractProperties(full)
}

// RawContentIndexMapping returns the full Elasticsearch index body (settings +
// mappings) for creating a raw_content index. Use this when creating indexes
// so that jsonld_raw and other fields use the canonical mapping (e.g. enabled:
// false for jsonld_raw to avoid dynamic mapping conflicts).
func RawContentIndexMapping() map[string]any {
	return mappings.GetRawContentMapping(1, 0)
}

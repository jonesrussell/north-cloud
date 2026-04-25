package contracts

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// ClassifiedContentMapping returns the canonical classified_content index
// mapping as a contract Mapping. Services that write to or read from
// *_classified_content indexes should test against this mapping.
func ClassifiedContentMapping() Mapping {
	full := esmapping.ClassifiedContentIndex(1, 0)
	return extractProperties(full)
}

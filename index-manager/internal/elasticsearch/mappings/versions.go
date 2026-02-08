package mappings

// Mapping version constants.
// Bump major for breaking changes (field type changes, removals).
// Bump minor for additions.
const (
	RawContentMappingVersion        = "2.0.0"
	ClassifiedContentMappingVersion = "2.0.0"
)

// GetMappingVersion returns the current mapping version for an index type.
func GetMappingVersion(indexType string) string {
	switch indexType {
	case "raw_content":
		return RawContentMappingVersion
	case "classified_content":
		return ClassifiedContentMappingVersion
	default:
		return "1.0.0"
	}
}

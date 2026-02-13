package mappings

import "fmt"

// GetMappingForType returns the appropriate mapping for an index type
func GetMappingForType(indexType string, shards, replicas int) (map[string]any, error) {
	switch indexType {
	case "raw_content":
		return GetRawContentMapping(shards, replicas), nil
	case "classified_content":
		return GetClassifiedContentMapping(shards, replicas), nil
	default:
		return nil, fmt.Errorf("unknown index type: %s", indexType)
	}
}

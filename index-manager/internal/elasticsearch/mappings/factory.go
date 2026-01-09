package mappings

import "fmt"

// GetMappingForType returns the appropriate mapping for an index type
func GetMappingForType(indexType string) (map[string]any, error) {
	switch indexType {
	case "raw_content":
		return GetRawContentMapping(), nil
	case "classified_content":
		return GetClassifiedContentMapping(), nil
	case "article":
		return GetArticleMapping(), nil
	case "page":
		return GetPageMapping(), nil
	default:
		return nil, fmt.Errorf("unknown index type: %s", indexType)
	}
}

package mappings

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// GetClassifiedContentMapping returns the Elasticsearch mapping for classified content indexes.
func GetClassifiedContentMapping(shards, replicas int) map[string]any {
	return esmapping.ClassifiedContentIndex(shards, replicas)
}

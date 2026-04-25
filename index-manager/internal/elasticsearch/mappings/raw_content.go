package mappings

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// GetRawContentMapping returns the Elasticsearch mapping for raw content indexes.
func GetRawContentMapping(shards, replicas int) map[string]any {
	return esmapping.RawContentIndex(shards, replicas)
}

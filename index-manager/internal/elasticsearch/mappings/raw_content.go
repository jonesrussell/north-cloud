package mappings

// GetRawContentMapping returns the Elasticsearch mapping for raw content indexes
func GetRawContentMapping(shards, replicas int) map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   shards,
			"number_of_replicas": replicas,
		},
		"mappings": map[string]any{
			"dynamic":    "strict",
			"properties": getRawContentFields(),
		},
	}
}

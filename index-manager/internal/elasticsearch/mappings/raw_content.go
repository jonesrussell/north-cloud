package mappings

// GetRawContentMapping returns the Elasticsearch mapping for raw content indexes
func GetRawContentMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]any{
			"properties": getRawContentFields(),
		},
	}
}

package esmapping

// RawContentIndex returns settings+mappings for a *_raw_content Elasticsearch index.
func RawContentIndex(shards, replicas int) map[string]any {
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

// RawContentProperties returns the top-level properties map for raw_content indices.
func RawContentProperties() map[string]any {
	return getRawContentFields()
}

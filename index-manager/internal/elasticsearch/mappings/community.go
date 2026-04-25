package mappings

import "github.com/jonesrussell/north-cloud/infrastructure/esmapping"

// getCommunityExternalIDsMapping returns the external_ids nested object mapping.
func getCommunityExternalIDsMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"inac": map[string]any{
				"type": "keyword",
			},
			"statcan": map[string]any{
				"type": "keyword",
			},
		},
	}
}

// getCommunityProperties returns the community field definitions.
func getCommunityProperties() map[string]any {
	return map[string]any{
		"id": map[string]any{
			"type": "keyword",
		},
		"name": map[string]any{
			"type":            "text",
			"analyzer":        "autocomplete",
			"search_analyzer": "autocomplete_search",
			"fields": map[string]any{
				"keyword": map[string]any{
					"type": "keyword",
				},
				"suggest": map[string]any{
					"type":             "search_as_you_type",
					"max_shingle_size": maxShingleSize,
				},
			},
		},
		"community_type": map[string]any{
			"type": "keyword",
		},
		"province": map[string]any{
			"type": "keyword",
		},
		"location": map[string]any{
			"type": "geo_point",
		},
		"population": map[string]any{
			"type": "integer",
		},
		"governing_body": map[string]any{
			"type": "keyword",
		},
		"external_ids": getCommunityExternalIDsMapping(),
		"region": map[string]any{
			"type": "keyword",
		},
		"subregion": map[string]any{
			"type": "keyword",
		},
		"neighbours": map[string]any{
			"type": "keyword",
		},
		"notes": map[string]any{
			"type":  "text",
			"index": false,
		},
		"created_at": map[string]any{
			"type":   "date",
			"format": esmapping.ESDateFormat,
		},
		"updated_at": map[string]any{
			"type":   "date",
			"format": esmapping.ESDateFormat,
		},
	}
}

// maxShingleSize is the max shingle size for search_as_you_type fields.
const maxShingleSize = 3

// getAutocompleteAnalysisSettings returns custom analyzer settings for community name autocomplete.
func getAutocompleteAnalysisSettings() map[string]any {
	return map[string]any{
		"analyzer": map[string]any{
			"autocomplete": map[string]any{
				"type":      "custom",
				"tokenizer": "standard",
				"filter":    []string{"lowercase", "autocomplete_edge"},
			},
			"autocomplete_search": map[string]any{
				"type":      "custom",
				"tokenizer": "standard",
				"filter":    []string{"lowercase"},
			},
		},
		"filter": map[string]any{
			"autocomplete_edge": map[string]any{
				"type":     "edge_ngram",
				"min_gram": minEdgeNgram,
				"max_gram": maxEdgeNgram,
			},
		},
	}
}

// Edge n-gram bounds for autocomplete tokenizer.
const (
	minEdgeNgram = 2
	maxEdgeNgram = 20
)

// GetCommunityMapping returns the Elasticsearch mapping for the community index.
func GetCommunityMapping(shards, replicas int) map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   shards,
			"number_of_replicas": replicas,
			"analysis":           getAutocompleteAnalysisSettings(),
		},
		"mappings": map[string]any{
			"dynamic":    "strict",
			"properties": getCommunityProperties(),
		},
	}
}

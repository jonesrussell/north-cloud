package esmapping

// EnglishAnalysisSettings returns custom English analyzer settings for classified_content.
func EnglishAnalysisSettings() map[string]any {
	return map[string]any{
		"analyzer": map[string]any{
			"english_content": map[string]any{
				"type":      "custom",
				"tokenizer": "standard",
				"filter":    []string{"lowercase", "english_stop", "english_stemmer"},
			},
		},
		"filter": map[string]any{
			"english_stop":    map[string]any{"type": "stop", "stopwords": "_english_"},
			"english_stemmer": map[string]any{"type": "stemmer", "language": "english"},
		},
	}
}

func setEnglishContentAnalyzer(properties map[string]any, field string) {
	if fieldMap, ok := properties[field].(map[string]any); ok {
		fieldMap["analyzer"] = "english_content"
	}
}

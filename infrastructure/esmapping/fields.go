package esmapping

// ESDateFormat is the standard Elasticsearch date format for pipeline timestamps.
const ESDateFormat = "strict_date_optional_time||epoch_millis"

// TextStandard returns a standard-analyzer text field mapping.
func TextStandard() map[string]any {
	return map[string]any{
		"type":     "text",
		"analyzer": "standard",
	}
}

// TextWithKeywordSubfield returns text + .keyword for faceted search.
func TextWithKeywordSubfield() map[string]any {
	return map[string]any{
		"type":     "text",
		"analyzer": "standard",
		"fields": map[string]any{
			"keyword": map[string]any{"type": "keyword"},
		},
	}
}

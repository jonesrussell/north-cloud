package esmapping

import "maps"

// getRawContentFields returns the raw content field definitions
func getRawContentFields() map[string]any {
	indexFalse := false
	return map[string]any{
		"id": map[string]any{
			"type": "keyword",
		},
		"url": map[string]any{
			"type": "keyword",
		},
		"source_name": map[string]any{
			"type": "keyword",
		},
		"title": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"raw_html": map[string]any{
			"type":  "text",
			"index": indexFalse, // Store but don't index
		},
		"raw_text": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"og_type": map[string]any{
			"type": "keyword",
		},
		"og_title": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"og_description": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"og_image": map[string]any{
			"type": "keyword",
		},
		"og_url": map[string]any{
			"type": "keyword",
		},
		"meta_description": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"meta_keywords": map[string]any{
			"type": "keyword",
		},
		"canonical_url": map[string]any{
			"type": "keyword",
		},
		"author": map[string]any{
			"type": "text",
		},
		"crawled_at": map[string]any{
			"type":   "date",
			"format": ESDateFormat,
		},
		"published_date": map[string]any{
			"type":   "date",
			"format": ESDateFormat,
		},
		"classification_status": map[string]any{
			"type": "keyword",
		},
		"classified_at": map[string]any{
			"type":   "date",
			"format": ESDateFormat,
		},
		"word_count": map[string]any{
			"type": "integer",
		},
		"article_section": map[string]any{
			"type": "keyword",
		},
		"json_ld_data": map[string]any{
			"type":       "object",
			"properties": getJSONLdDataFields(),
		},
		"meta": map[string]any{
			"type":       "object",
			"properties": getMetaFields(),
		},
	}
}

// getJSONLdDataFields returns the JSON-LD extracted data field definitions
func getJSONLdDataFields() map[string]any {
	return map[string]any{
		"jsonld_headline":        TextStandard(),
		"jsonld_description":     map[string]any{"type": "text"},
		"jsonld_article_section": map[string]any{"type": "keyword"},
		"jsonld_author":          map[string]any{"type": "text"},
		"jsonld_publisher_name":  map[string]any{"type": "text"},
		"jsonld_url":             map[string]any{"type": "keyword"},
		"jsonld_image_url":       map[string]any{"type": "keyword"},
		"jsonld_date_published":  map[string]any{"type": "date", "format": ESDateFormat},
		"jsonld_date_created":    map[string]any{"type": "date", "format": ESDateFormat},
		"jsonld_date_modified":   map[string]any{"type": "date", "format": ESDateFormat},
		"jsonld_word_count":      map[string]any{"type": "integer"},
		"jsonld_keywords":        map[string]any{"type": "keyword"},
		"jsonld_schema_type":     map[string]any{"type": "keyword"},
		"jsonld_location":        map[string]any{"type": "text"},
		"jsonld_raw":             map[string]any{"type": "object", "enabled": false},
	}
}

// getMetaFields returns the meta tag field definitions (union of crawler/classifier
// and index-manager historical shapes). article_opinion uses keyword so string
// heuristics and booleans can coexist; see docs/generated/es-mapping-divergence.md.
func getMetaFields() map[string]any {
	textKW := TextWithKeywordSubfield()
	return map[string]any{
		"twitter_card":          map[string]any{"type": "keyword"},
		"twitter_site":          map[string]any{"type": "keyword"},
		"og_image_width":        map[string]any{"type": "integer"},
		"og_image_height":       map[string]any{"type": "integer"},
		"og_site_name":          map[string]any{"type": "keyword"},
		"created_at":            map[string]any{"type": "date", "format": ESDateFormat},
		"updated_at":            map[string]any{"type": "date", "format": ESDateFormat},
		"article_opinion":       map[string]any{"type": "keyword"},
		"article_content_tier":  map[string]any{"type": "keyword"},
		"detected_content_type": textKW,
		"page_type":             textKW,
		"indigenous_region":     textKW,
	}
}

// getCrimeMapping returns the nested crime object mapping (union of index-manager
// contract fields and classifier CrimeResult JSON field names).
func getCrimeMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sub_label": map[string]any{
				"type": "keyword",
			},
			"primary_crime_type": map[string]any{
				"type": "keyword",
			},
			"relevance": map[string]any{
				"type": "keyword",
			},
			"street_crime_relevance": map[string]any{
				"type": "keyword",
			},
			"crime_types": map[string]any{
				"type": "keyword",
			},
			"location_specificity": map[string]any{
				"type": "keyword",
			},
			"category_pages": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"homepage_eligible": map[string]any{
				"type": "boolean",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
			"decision_path": map[string]any{
				"type": "keyword",
			},
			"ml_confidence_raw": map[string]any{
				"type": "float",
			},
			"rule_triggered": map[string]any{
				"type": "keyword",
			},
			"processing_time_ms": map[string]any{
				"type": "long",
			},
		},
	}
}

// getLocationMapping returns the nested location object mapping
func getLocationMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type": "keyword",
			},
			"province": map[string]any{
				"type": "keyword",
			},
			"country": map[string]any{
				"type": "keyword",
			},
			"specificity": map[string]any{
				"type": "keyword",
			},
			"confidence": map[string]any{
				"type": "float",
			},
		},
	}
}

// getMiningMapping returns the nested mining object mapping
func getMiningMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance": map[string]any{
				"type": "keyword",
			},
			"mining_stage": map[string]any{
				"type": "keyword",
			},
			"commodities": map[string]any{
				"type": "keyword",
			},
			"location": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
			"extraction_method": map[string]any{
				"type": "keyword",
			},
			"drill_results": map[string]any{
				"type": "nested",
				"properties": map[string]any{
					"hole_id":     map[string]any{"type": "keyword"},
					"commodity":   map[string]any{"type": "keyword"},
					"intercept_m": map[string]any{"type": "float"},
					"grade":       map[string]any{"type": "float"},
					"unit":        map[string]any{"type": "keyword"},
				},
			},
			"decision_path": map[string]any{
				"type": "keyword",
			},
			"ml_confidence_raw": map[string]any{
				"type": "float",
			},
			"rule_triggered": map[string]any{
				"type": "keyword",
			},
			"processing_time_ms": map[string]any{
				"type": "long",
			},
		},
	}
}

// getIndigenousMapping returns the nested indigenous object mapping
func getIndigenousMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance": map[string]any{
				"type": "keyword",
			},
			"categories": map[string]any{
				"type": "keyword",
			},
			"region": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
			"decision_path": map[string]any{
				"type": "keyword",
			},
			"ml_confidence_raw": map[string]any{
				"type": "float",
			},
			"rule_triggered": map[string]any{
				"type": "keyword",
			},
			"processing_time_ms": map[string]any{
				"type": "long",
			},
		},
	}
}

// getCoforgeMapping returns the nested coforge object mapping
func getCoforgeMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance": map[string]any{
				"type": "keyword",
			},
			"relevance_confidence": map[string]any{
				"type": "float",
			},
			"audience": map[string]any{
				"type": "keyword",
			},
			"audience_confidence": map[string]any{
				"type": "float",
			},
			"topics": map[string]any{
				"type": "keyword",
			},
			"industries": map[string]any{
				"type": "keyword",
			},
			"final_confidence": map[string]any{
				"type": "float",
			},
			"review_required": map[string]any{
				"type": "boolean",
			},
			"model_version": map[string]any{
				"type": "keyword",
			},
			"decision_path": map[string]any{
				"type": "keyword",
			},
			"ml_confidence_raw": map[string]any{
				"type": "float",
			},
			"rule_triggered": map[string]any{
				"type": "keyword",
			},
			"processing_time_ms": map[string]any{
				"type": "long",
			},
		},
	}
}

// getRecipeMapping returns the nested recipe object mapping
func getRecipeMapping() map[string]any {
	indexFalse := false
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"extraction_method":  map[string]any{"type": "keyword"},
			"name":               map[string]any{"type": "text", "analyzer": "standard"},
			"ingredients":        map[string]any{"type": "text", "analyzer": "standard"},
			"instructions":       map[string]any{"type": "text", "analyzer": "standard"},
			"prep_time_minutes":  map[string]any{"type": "integer"},
			"cook_time_minutes":  map[string]any{"type": "integer"},
			"total_time_minutes": map[string]any{"type": "integer"},
			"servings":           map[string]any{"type": "keyword"},
			"category":           map[string]any{"type": "keyword"},
			"cuisine":            map[string]any{"type": "keyword"},
			"calories":           map[string]any{"type": "keyword"},
			"image_url":          map[string]any{"type": "keyword", "index": indexFalse},
			"rating":             map[string]any{"type": "float"},
			"rating_count":       map[string]any{"type": "integer"},
			"decision_path":      map[string]any{"type": "keyword"},
			"ml_confidence_raw":  map[string]any{"type": "float"},
			"rule_triggered":     map[string]any{"type": "keyword"},
			"processing_time_ms": map[string]any{"type": "long"},
		},
	}
}

// getJobMapping returns the nested job object mapping
func getJobMapping() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"extraction_method":  map[string]any{"type": "keyword"},
			"title":              map[string]any{"type": "text", "analyzer": "standard"},
			"company":            map[string]any{"type": "keyword"},
			"location":           map[string]any{"type": "keyword"},
			"salary_min":         map[string]any{"type": "float"},
			"salary_max":         map[string]any{"type": "float"},
			"salary_currency":    map[string]any{"type": "keyword"},
			"employment_type":    map[string]any{"type": "keyword"},
			"posted_date":        map[string]any{"type": "date", "format": ESDateFormat},
			"expires_date":       map[string]any{"type": "date", "format": ESDateFormat},
			"description":        map[string]any{"type": "text", "analyzer": "standard"},
			"industry":           map[string]any{"type": "keyword"},
			"qualifications":     map[string]any{"type": "text", "analyzer": "standard"},
			"benefits":           map[string]any{"type": "text", "analyzer": "standard"},
			"decision_path":      map[string]any{"type": "keyword"},
			"ml_confidence_raw":  map[string]any{"type": "float"},
			"rule_triggered":     map[string]any{"type": "keyword"},
			"processing_time_ms": map[string]any{"type": "long"},
		},
	}
}

func getEntertainmentClassifierNested() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"relevance":          map[string]any{"type": "keyword"},
			"categories":         map[string]any{"type": "keyword"},
			"final_confidence":   map[string]any{"type": "float"},
			"homepage_eligible":  map[string]any{"type": "boolean"},
			"review_required":    map[string]any{"type": "boolean"},
			"model_version":      map[string]any{"type": "keyword"},
			"decision_path":      map[string]any{"type": "keyword"},
			"ml_confidence_raw":  map[string]any{"type": "float"},
			"rule_triggered":     map[string]any{"type": "keyword"},
			"processing_time_ms": map[string]any{"type": "long"},
		},
	}
}

func getRFPClassifierNested() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"extraction_method": map[string]any{"type": "keyword"},
			"document_type":     map[string]any{"type": "keyword"},
			"title":             map[string]any{"type": "text", "analyzer": "standard"},
			"reference_number":  map[string]any{"type": "keyword"},
			"organization_name": map[string]any{"type": "keyword"},
			"description":       map[string]any{"type": "text", "analyzer": "standard"},
			"published_date":    map[string]any{"type": "keyword"},
			"closing_date":      map[string]any{"type": "keyword"},
			"amendment_date":    map[string]any{"type": "keyword"},
			"budget_min":        map[string]any{"type": "float"},
			"budget_max":        map[string]any{"type": "float"},
			"budget_currency":   map[string]any{"type": "keyword"},
			"procurement_type":  map[string]any{"type": "keyword"},
			"naics_codes":       map[string]any{"type": "keyword"},
			"categories":        map[string]any{"type": "keyword"},
			"province":          map[string]any{"type": "keyword"},
			"city":              map[string]any{"type": "keyword"},
			"country":           map[string]any{"type": "keyword"},
			"eligibility":       map[string]any{"type": "text", "analyzer": "standard"},
			"source_url":        map[string]any{"type": "keyword"},
			"contact_name":      map[string]any{"type": "keyword"},
			"contact_email":     map[string]any{"type": "keyword"},
		},
	}
}

func getNeedSignalClassifierNested() map[string]any {
	orgName := map[string]any{
		"type":     "text",
		"analyzer": "standard",
		"fields": map[string]any{
			"keyword": map[string]any{"type": "keyword"},
		},
	}
	city := map[string]any{
		"type":     "text",
		"analyzer": "standard",
		"fields": map[string]any{
			"keyword": map[string]any{"type": "keyword"},
		},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"signal_type":                  map[string]any{"type": "keyword"},
			"organization_name":            orgName,
			"organization_name_normalized": map[string]any{"type": "keyword"},
			"sector":                       map[string]any{"type": "keyword"},
			"province":                     map[string]any{"type": "keyword"},
			"city":                         city,
			"contact_email":                map[string]any{"type": "keyword"},
			"contact_name":                 map[string]any{"type": "text", "analyzer": "standard"},
			"source_url":                   map[string]any{"type": "keyword"},
			"keywords":                     map[string]any{"type": "keyword"},
			"confidence":                   map[string]any{"type": "float"},
		},
	}
}

// getClassificationFields returns the classification result field definitions
func getClassificationFields() map[string]any {
	return map[string]any{
		"content_type": map[string]any{
			"type": "text",
			"fields": map[string]any{
				"keyword": map[string]any{"type": "keyword"},
			},
		},
		"content_subtype": map[string]any{
			"type": "keyword",
		},
		"type_confidence": map[string]any{
			"type": "float",
		},
		"type_method": map[string]any{
			"type": "keyword",
		},
		"quality_score": map[string]any{
			"type": "integer",
		},
		"quality_factors": map[string]any{
			"type": "object",
		},
		"topics": map[string]any{
			"type": "keyword",
		},
		"topic_scores": map[string]any{
			"type": "object",
		},
		"crime":         getCrimeMapping(),
		"location":      getLocationMapping(),
		"mining":        getMiningMapping(),
		"coforge":       getCoforgeMapping(),
		"indigenous":    getIndigenousMapping(),
		"recipe":        getRecipeMapping(),
		"job":           getJobMapping(),
		"entertainment": getEntertainmentClassifierNested(),
		"rfp":           getRFPClassifierNested(),
		"need_signal":   getNeedSignalClassifierNested(),
		"low_quality": map[string]any{
			"type": "boolean",
		},
		"body": map[string]any{
			"type":     "text",
			"analyzer": "standard",
		},
		"source": map[string]any{
			"type": "keyword",
		},
		"source_reputation": map[string]any{
			"type": "integer",
		},
		"source_category": map[string]any{
			"type": "keyword",
		},
		"classifier_version": map[string]any{
			"type": "keyword",
		},
		"classification_method": map[string]any{
			"type": "keyword",
		},
		"model_version": map[string]any{
			"type": "keyword",
		},
		"confidence": map[string]any{
			"type": "float",
		},
		"processing_time_ms": map[string]any{
			"type": "long",
		},
	}
}

// ClassifiedContentIndex returns the Elasticsearch mapping for classified content indexes.
func ClassifiedContentIndex(shards, replicas int) map[string]any {
	properties := make(map[string]any)

	// Add raw content fields
	maps.Copy(properties, getRawContentFields())

	// Add classification fields
	maps.Copy(properties, getClassificationFields())

	// Override text fields to use english_content analyzer for search quality
	setEnglishContentAnalyzer(properties, "title")
	setEnglishContentAnalyzer(properties, "raw_text")
	setEnglishContentAnalyzer(properties, "body")
	setEnglishContentAnalyzer(properties, "content_type")

	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   shards,
			"number_of_replicas": replicas,
			"analysis":           EnglishAnalysisSettings(),
		},
		"mappings": map[string]any{
			"dynamic":    "strict",
			"properties": properties,
		},
	}
}

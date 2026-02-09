package mappings_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
)

// --- DefaultSettings ---

func TestDefaultSettings(t *testing.T) {
	t.Helper()

	settings := mappings.DefaultSettings()

	if settings.NumberOfShards != 1 {
		t.Errorf("NumberOfShards = %d, want 1", settings.NumberOfShards)
	}
	if settings.NumberOfReplicas != 1 {
		t.Errorf("NumberOfReplicas = %d, want 1", settings.NumberOfReplicas)
	}
}

// --- ToMap ---

func TestToMap_RoundTrip(t *testing.T) {
	t.Helper()

	input := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 42,
	}

	result, err := mappings.ToMap(input)
	if err != nil {
		t.Fatalf("ToMap() error = %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("result[name] = %v, want %q", result["name"], "test")
	}
	// JSON numbers decode as float64
	if result["value"] != float64(42) {
		t.Errorf("result[value] = %v, want %v", result["value"], float64(42))
	}
}

// --- Factory ---

func TestGetMappingForType_ValidTypes(t *testing.T) {
	t.Helper()

	types := []string{"raw_content", "classified_content", "article", "page"}

	for _, indexType := range types {
		t.Run(indexType, func(t *testing.T) {
			mapping, err := mappings.GetMappingForType(indexType, 1, 1)
			if err != nil {
				t.Fatalf("GetMappingForType(%q) error = %v", indexType, err)
			}
			if mapping == nil {
				t.Fatalf("GetMappingForType(%q) returned nil", indexType)
			}
			if _, ok := mapping["settings"]; !ok {
				t.Errorf("GetMappingForType(%q) missing 'settings' key", indexType)
			}
			if _, ok := mapping["mappings"]; !ok {
				t.Errorf("GetMappingForType(%q) missing 'mappings' key", indexType)
			}
		})
	}
}

func TestGetMappingForType_UnknownType(t *testing.T) {
	t.Helper()

	_, err := mappings.GetMappingForType("nonexistent", 1, 1)
	if err == nil {
		t.Fatal("GetMappingForType(nonexistent) = nil error, want error")
	}
}

// --- Raw Content Mapping ---

func TestGetRawContentMapping_Structure(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)

	settings, ok := mapping["settings"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid settings")
	}
	if settings["number_of_shards"] != 1 {
		t.Errorf("number_of_shards = %v, want 1", settings["number_of_shards"])
	}

	mappingsObj, ok := mapping["mappings"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid mappings")
	}
	properties, ok := mappingsObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid properties")
	}

	expectedFields := []string{
		"id", "url", "source_name", "title", "raw_html", "raw_text",
		"og_type", "og_title", "og_description", "og_image", "og_url",
		"meta_description", "meta_keywords", "canonical_url", "author",
		"crawled_at", "published_date", "classification_status", "classified_at",
		"word_count", "article_section", "json_ld_data", "meta",
	}

	for _, field := range expectedFields {
		if _, exists := properties[field]; !exists {
			t.Errorf("raw_content mapping missing field %q", field)
		}
	}

	expectedFieldCount := 23
	if len(properties) != expectedFieldCount {
		t.Errorf("raw_content has %d fields, want %d", len(properties), expectedFieldCount)
	}
}

func TestGetRawContentMapping_FieldTypes(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	keywordFields := []string{
		"id", "url", "source_name", "og_type", "og_image", "og_url",
		"meta_keywords", "canonical_url", "classification_status",
	}
	for _, field := range keywordFields {
		assertFieldType(t, properties, field, "keyword")
	}

	textAnalyzedFields := []string{"title", "raw_text", "og_title", "og_description", "meta_description"}
	for _, field := range textAnalyzedFields {
		assertFieldType(t, properties, field, "text")
		assertFieldHasAnalyzer(t, properties, field, "standard")
	}

	dateFields := []string{"crawled_at", "published_date", "classified_at"}
	for _, field := range dateFields {
		assertFieldType(t, properties, field, "date")
		assertFieldFormat(t, properties, field, "strict_date_optional_time||epoch_millis")
	}

	assertFieldType(t, properties, "word_count", "integer")
}

func TestGetRawContentMapping_RawHTMLNotIndexed(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	rawHTML, ok := properties["raw_html"].(map[string]any)
	if !ok {
		t.Fatal("raw_html field missing")
	}

	indexVal, exists := rawHTML["index"]
	if !exists {
		t.Fatal("raw_html missing 'index' property")
	}
	if indexVal != false {
		t.Errorf("raw_html index = %v, want false", indexVal)
	}
}

func TestGetRawContentMapping_DynamicStrict(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)
	mappingsObj := mapping["mappings"].(map[string]any)

	dynamic, exists := mappingsObj["dynamic"]
	if !exists {
		t.Fatal("raw_content mapping missing 'dynamic' setting")
	}
	if dynamic != "strict" {
		t.Errorf("dynamic = %v, want \"strict\"", dynamic)
	}
}

func TestGetClassifiedContentMapping_DynamicStrict(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	mappingsObj := mapping["mappings"].(map[string]any)

	dynamic, exists := mappingsObj["dynamic"]
	if !exists {
		t.Fatal("classified_content mapping missing 'dynamic' setting")
	}
	if dynamic != "strict" {
		t.Errorf("dynamic = %v, want \"strict\"", dynamic)
	}
}

func TestGetRawContentMapping_MetaSubFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	metaObj, ok := properties["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta field missing or not an object")
	}
	metaProps, ok := metaObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("meta.properties missing")
	}

	expectedMetaFields := []string{
		"twitter_card", "twitter_site", "og_image_width", "og_image_height",
		"og_site_name", "created_at", "updated_at", "article_opinion", "article_content_tier",
	}
	for _, field := range expectedMetaFields {
		if _, exists := metaProps[field]; !exists {
			t.Errorf("meta missing field %q", field)
		}
	}
}

func TestGetRawContentMapping_JsonLdDataSubFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetRawContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	jsonLdObj, ok := properties["json_ld_data"].(map[string]any)
	if !ok {
		t.Fatal("json_ld_data field missing or not an object")
	}
	jsonLdProps, ok := jsonLdObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("json_ld_data.properties missing")
	}

	expectedJSONLdFields := []string{
		"jsonld_headline", "jsonld_description", "jsonld_article_section",
		"jsonld_author", "jsonld_publisher_name", "jsonld_url", "jsonld_image_url",
		"jsonld_date_published", "jsonld_date_created", "jsonld_date_modified",
		"jsonld_word_count", "jsonld_keywords", "jsonld_raw",
	}
	for _, field := range expectedJSONLdFields {
		if _, exists := jsonLdProps[field]; !exists {
			t.Errorf("json_ld_data missing field %q", field)
		}
	}
}

// --- Classified Content Mapping ---

func TestGetClassifiedContentMapping_InheritsRawFields(t *testing.T) {
	t.Helper()

	rawMapping := mappings.GetRawContentMapping(1, 1)
	classifiedMapping := mappings.GetClassifiedContentMapping(1, 1)

	rawProps := rawMapping["mappings"].(map[string]any)["properties"].(map[string]any)
	classifiedProps := classifiedMapping["mappings"].(map[string]any)["properties"].(map[string]any)

	for field := range rawProps {
		if _, exists := classifiedProps[field]; !exists {
			t.Errorf("classified_content missing raw field %q", field)
		}
	}
}

func TestGetClassifiedContentMapping_ClassificationFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	classificationFields := []string{
		"content_type", "content_subtype", "quality_score", "quality_factors",
		"topics", "topic_scores", "crime", "location", "mining", "coforge",
		"source_reputation", "source_category",
		"classifier_version", "classification_method", "model_version", "confidence",
	}

	for _, field := range classificationFields {
		if _, exists := properties[field]; !exists {
			t.Errorf("classified_content missing classification field %q", field)
		}
	}
}

func TestGetClassifiedContentMapping_NestedCrimeFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	crimeObj, ok := properties["crime"].(map[string]any)
	if !ok {
		t.Fatal("crime field missing or not an object")
	}
	crimeProps, ok := crimeObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("crime.properties missing")
	}

	expectedCrimeFields := []string{
		"sub_label", "primary_crime_type", "relevance", "crime_types",
		"final_confidence", "homepage_eligible", "review_required", "model_version",
	}
	for _, field := range expectedCrimeFields {
		if _, exists := crimeProps[field]; !exists {
			t.Errorf("crime missing field %q", field)
		}
	}
}

func TestGetClassifiedContentMapping_NestedLocationFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	locationObj, ok := properties["location"].(map[string]any)
	if !ok {
		t.Fatal("location field missing or not an object")
	}
	locationProps, ok := locationObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("location.properties missing")
	}

	expectedLocationFields := []string{"city", "province", "country", "specificity", "confidence"}
	for _, field := range expectedLocationFields {
		if _, exists := locationProps[field]; !exists {
			t.Errorf("location missing field %q", field)
		}
	}
}

func TestGetClassifiedContentMapping_NestedMiningFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	miningObj, ok := properties["mining"].(map[string]any)
	if !ok {
		t.Fatal("mining field missing or not an object")
	}
	miningProps, ok := miningObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("mining.properties missing")
	}

	expectedMiningFields := []string{
		"relevance", "mining_stage", "commodities", "location",
		"final_confidence", "review_required", "model_version",
	}
	for _, field := range expectedMiningFields {
		if _, exists := miningProps[field]; !exists {
			t.Errorf("mining missing field %q", field)
		}
	}
}

func TestGetClassifiedContentMapping_NestedCoforgeFields(t *testing.T) {
	t.Helper()

	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)

	coforgeObj, ok := properties["coforge"].(map[string]any)
	if !ok {
		t.Fatal("coforge field missing or not an object")
	}
	coforgeProps, ok := coforgeObj["properties"].(map[string]any)
	if !ok {
		t.Fatal("coforge.properties missing")
	}

	expectedCoforgeFields := []string{
		"relevance", "relevance_confidence", "audience", "audience_confidence",
		"topics", "industries", "final_confidence", "review_required", "model_version",
	}
	for _, field := range expectedCoforgeFields {
		if _, exists := coforgeProps[field]; !exists {
			t.Errorf("coforge missing field %q", field)
		}
	}
}

// --- English Analyzer ---

func TestGetClassifiedContentMapping_HasEnglishAnalyzer(t *testing.T) {
	t.Helper()
	mapping := mappings.GetClassifiedContentMapping(1, 1)
	settings := mapping["settings"].(map[string]any)

	analysis, hasAnalysis := settings["analysis"]
	if !hasAnalysis {
		t.Fatal("classified_content mapping missing 'analysis' settings")
	}
	analysisMap := analysis.(map[string]any)

	analyzer, hasAnalyzer := analysisMap["analyzer"]
	if !hasAnalyzer {
		t.Fatal("missing analyzer in analysis settings")
	}
	analyzerMap := analyzer.(map[string]any)

	if _, hasEnglish := analyzerMap["english_content"]; !hasEnglish {
		t.Error("missing english_content analyzer")
	}
}

func TestGetClassifiedContentMapping_TextFieldsUseEnglishAnalyzer(t *testing.T) {
	t.Helper()
	mapping := mappings.GetClassifiedContentMapping(1, 1)
	properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)
	assertFieldHasAnalyzer(t, properties, "title", "english_content")
	assertFieldHasAnalyzer(t, properties, "raw_text", "english_content")
}

// --- Version Constants ---

func TestMappingVersionConstants(t *testing.T) {
	t.Helper()

	if mappings.RawContentMappingVersion == "" {
		t.Error("RawContentMappingVersion is empty")
	}
	if mappings.ClassifiedContentMappingVersion == "" {
		t.Error("ClassifiedContentMappingVersion is empty")
	}
}

func TestGetMappingVersion(t *testing.T) {
	t.Helper()

	if v := mappings.GetMappingVersion("raw_content"); v != mappings.RawContentMappingVersion {
		t.Errorf("GetMappingVersion(raw_content) = %q, want %q", v, mappings.RawContentMappingVersion)
	}
	if v := mappings.GetMappingVersion("classified_content"); v != mappings.ClassifiedContentMappingVersion {
		t.Errorf("GetMappingVersion(classified_content) = %q, want %q", v, mappings.ClassifiedContentMappingVersion)
	}
	if v := mappings.GetMappingVersion("unknown"); v != "1.0.0" {
		t.Errorf("GetMappingVersion(unknown) = %q, want \"1.0.0\"", v)
	}
}

// --- Helpers ---

func assertFieldType(t *testing.T, properties map[string]any, field, expectedType string) {
	t.Helper()

	fieldMap, ok := properties[field].(map[string]any)
	if !ok {
		t.Errorf("field %q missing or not a map", field)
		return
	}
	if fieldMap["type"] != expectedType {
		t.Errorf("field %q type = %v, want %q", field, fieldMap["type"], expectedType)
	}
}

func assertFieldHasAnalyzer(t *testing.T, properties map[string]any, field, expectedAnalyzer string) {
	t.Helper()

	fieldMap := properties[field].(map[string]any)
	if fieldMap["analyzer"] != expectedAnalyzer {
		t.Errorf("field %q analyzer = %v, want %q", field, fieldMap["analyzer"], expectedAnalyzer)
	}
}

func assertFieldFormat(t *testing.T, properties map[string]any, field, expectedFormat string) {
	t.Helper()

	fieldMap := properties[field].(map[string]any)
	if fieldMap["format"] != expectedFormat {
		t.Errorf("field %q format = %v, want %q", field, fieldMap["format"], expectedFormat)
	}
}

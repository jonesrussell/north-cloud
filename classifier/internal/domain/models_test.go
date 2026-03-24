package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constants tests ---

func TestContentTypeConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "article", domain.ContentTypeArticle)
	assert.Equal(t, "page", domain.ContentTypePage)
	assert.Equal(t, "video", domain.ContentTypeVideo)
	assert.Equal(t, "image", domain.ContentTypeImage)
	assert.Equal(t, "job", domain.ContentTypeJob)
	assert.Equal(t, "recipe", domain.ContentTypeRecipe)
	assert.Equal(t, "event", domain.ContentTypeEvent)
	assert.Equal(t, "obituary", domain.ContentTypeObituary)
	assert.Equal(t, "rfp", domain.ContentTypeRFP)
}

func TestContentSubtypeConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "press_release", domain.ContentSubtypePressRelease)
	assert.Equal(t, "blog_post", domain.ContentSubtypeBlogPost)
	assert.Equal(t, "event", domain.ContentSubtypeEvent)
	assert.Equal(t, "advisory", domain.ContentSubtypeAdvisory)
	assert.Equal(t, "report", domain.ContentSubtypeReport)
	assert.Equal(t, "blotter", domain.ContentSubtypeBlotter)
	assert.Equal(t, "company_announcement", domain.ContentSubtypeCompanyAnnouncement)
	assert.Equal(t, "event_report", domain.ContentSubtypeEventReport)
}

func TestSourceCategoryConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "news", domain.SourceCategoryNews)
	assert.Equal(t, "blog", domain.SourceCategoryBlog)
	assert.Equal(t, "government", domain.SourceCategoryGovernment)
	assert.Equal(t, "unknown", domain.SourceCategoryUnknown)
}

func TestClassificationMethodConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "rule_based", domain.MethodRuleBased)
	assert.Equal(t, "ml_model", domain.MethodMLModel)
	assert.Equal(t, "hybrid", domain.MethodHybrid)
}

func TestSpecificityConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "city", domain.SpecificityCity)
	assert.Equal(t, "province", domain.SpecificityProvince)
	assert.Equal(t, "country", domain.SpecificityCountry)
	assert.Equal(t, "unknown", domain.SpecificityUnknown)
}

func TestClassificationStatusConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "pending", domain.StatusPending)
	assert.Equal(t, "classified", domain.StatusClassified)
	assert.Equal(t, "failed", domain.StatusFailed)
}

func TestRuleTypeConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, "content_type", domain.RuleTypeContentType)
	assert.Equal(t, "topic", domain.RuleTypeTopic)
	assert.Equal(t, "quality", domain.RuleTypeQuality)
}

// --- RawContent tests ---

func TestRawContent_JSONRoundTrip(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	pubDate := now.Add(-24 * time.Hour)
	raw := domain.RawContent{
		ID:                   "doc-1",
		URL:                  "https://example.com/article",
		SourceName:           "example",
		Title:                "Test Article",
		RawText:              "Some body text here",
		OGType:               "article",
		OGTitle:              "OG Title",
		MetaDescription:      "A description",
		CrawledAt:            now,
		PublishedDate:        &pubDate,
		ClassificationStatus: domain.StatusPending,
		WordCount:            150,
		Meta:                 map[string]any{"detected_content_type": "article"},
	}

	data, err := json.Marshal(raw)
	require.NoError(t, err)

	var got domain.RawContent
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, raw.ID, got.ID)
	assert.Equal(t, raw.URL, got.URL)
	assert.Equal(t, raw.SourceName, got.SourceName)
	assert.Equal(t, raw.Title, got.Title)
	assert.Equal(t, raw.RawText, got.RawText)
	assert.Equal(t, raw.ClassificationStatus, got.ClassificationStatus)
	assert.Equal(t, raw.WordCount, got.WordCount)
}

func TestRawContent_SourceIndexOmittedFromJSON(t *testing.T) {
	t.Helper()

	raw := domain.RawContent{
		ID:          "doc-1",
		SourceIndex: "cbc_raw_content",
		SourceName:  "cbc",
	}

	data, err := json.Marshal(raw)
	require.NoError(t, err)

	// SourceIndex has json:"-" tag, should not appear in output
	assert.NotContains(t, string(data), "source_index")
	assert.NotContains(t, string(data), "cbc_raw_content")
}

// --- ClassificationRule tests ---

func TestClassificationRule_JSONRoundTrip(t *testing.T) {
	t.Helper()

	rule := domain.ClassificationRule{
		ID:            1,
		RuleName:      "crime_detection",
		RuleType:      domain.RuleTypeTopic,
		TopicName:     "violent_crime",
		Keywords:      []string{"murder", "assault", "shooting"},
		MinConfidence: 0.8,
		Enabled:       true,
		Priority:      100,
	}

	data, err := json.Marshal(rule)
	require.NoError(t, err)

	var got domain.ClassificationRule
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, rule.ID, got.ID)
	assert.Equal(t, rule.RuleName, got.RuleName)
	assert.Equal(t, rule.RuleType, got.RuleType)
	assert.Equal(t, rule.TopicName, got.TopicName)
	assert.Equal(t, rule.Keywords, got.Keywords)
	assert.InDelta(t, rule.MinConfidence, got.MinConfidence, 0.001)
	assert.Equal(t, rule.Enabled, got.Enabled)
	assert.Equal(t, rule.Priority, got.Priority)
}

// --- SourceReputation tests ---

func TestSourceReputation_JSONRoundTrip(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	rep := domain.SourceReputation{
		ID:                  1,
		SourceName:          "cbc",
		SourceURL:           "https://cbc.ca",
		Category:            domain.SourceCategoryNews,
		ReputationScore:     85,
		TotalArticles:       1000,
		AverageQualityScore: 72.5,
		SpamCount:           5,
		LastClassifiedAt:    &now,
	}

	data, err := json.Marshal(rep)
	require.NoError(t, err)

	var got domain.SourceReputation
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, rep.SourceName, got.SourceName)
	assert.Equal(t, rep.Category, got.Category)
	assert.Equal(t, rep.ReputationScore, got.ReputationScore)
	assert.Equal(t, rep.TotalArticles, got.TotalArticles)
	assert.InDelta(t, rep.AverageQualityScore, got.AverageQualityScore, 0.01)
	assert.Equal(t, rep.SpamCount, got.SpamCount)
}

// --- ClassifiedContent tests ---

func TestClassifiedContent_PublisherAliases(t *testing.T) {
	t.Helper()

	content := domain.ClassifiedContent{
		RawContent: domain.RawContent{
			RawText: "article body",
			URL:     "https://example.com/article",
		},
		Body:   "article body",
		Source: "https://example.com/article",
	}

	// Publisher expects Body and Source fields
	assert.Equal(t, content.RawText, content.Body)
	assert.Equal(t, content.URL, content.Source)
}

func TestClassifiedContent_OptionalFieldsNil(t *testing.T) {
	t.Helper()

	content := domain.ClassifiedContent{
		ContentType:  domain.ContentTypeArticle,
		QualityScore: 75,
	}

	data, err := json.Marshal(content)
	require.NoError(t, err)

	// Optional fields should be omitted when nil
	s := string(data)
	assert.NotContains(t, s, `"crime"`)
	assert.NotContains(t, s, `"mining"`)
	assert.NotContains(t, s, `"coforge"`)
	assert.NotContains(t, s, `"entertainment"`)
	assert.NotContains(t, s, `"indigenous"`)
	assert.NotContains(t, s, `"location"`)
	assert.NotContains(t, s, `"recipe"`)
	assert.NotContains(t, s, `"job"`)
	assert.NotContains(t, s, `"rfp"`)
}

func TestClassifiedContent_WithCrimeResult(t *testing.T) {
	t.Helper()

	content := domain.ClassifiedContent{
		ContentType: domain.ContentTypeArticle,
		Crime: &domain.CrimeResult{
			Relevance:           "core_street_crime",
			CrimeTypes:          []string{"violent_crime"},
			LocationSpecificity: "city",
			FinalConfidence:     0.92,
			HomepageEligible:    true,
			CategoryPages:       []string{"violent-crime", "crime"},
		},
	}

	data, err := json.Marshal(content)
	require.NoError(t, err)

	var got domain.ClassifiedContent
	require.NoError(t, json.Unmarshal(data, &got))

	require.NotNil(t, got.Crime)
	assert.Equal(t, "core_street_crime", got.Crime.Relevance)
	assert.Equal(t, []string{"violent_crime"}, got.Crime.CrimeTypes)
	assert.True(t, got.Crime.HomepageEligible)
	assert.Equal(t, []string{"violent-crime", "crime"}, got.Crime.CategoryPages)
}

func TestClassifiedContent_WithMiningResult(t *testing.T) {
	t.Helper()

	content := domain.ClassifiedContent{
		ContentType: domain.ContentTypeArticle,
		Mining: &domain.MiningResult{
			Relevance:       "core_mining",
			MiningStage:     "exploration",
			Commodities:     []string{"gold", "copper"},
			Location:        "local_canada",
			FinalConfidence: 0.88,
		},
	}

	data, err := json.Marshal(content)
	require.NoError(t, err)

	var got domain.ClassifiedContent
	require.NoError(t, json.Unmarshal(data, &got))

	require.NotNil(t, got.Mining)
	assert.Equal(t, "core_mining", got.Mining.Relevance)
	assert.Equal(t, "exploration", got.Mining.MiningStage)
	assert.Equal(t, []string{"gold", "copper"}, got.Mining.Commodities)
}

func TestClassifiedContent_WithIndigenousResult(t *testing.T) {
	t.Helper()

	content := domain.ClassifiedContent{
		ContentType: domain.ContentTypeArticle,
		Indigenous: &domain.IndigenousResult{
			Relevance:       "core_indigenous",
			Categories:      []string{"culture", "language"},
			Region:          "ontario",
			FinalConfidence: 0.91,
		},
	}

	data, err := json.Marshal(content)
	require.NoError(t, err)

	var got domain.ClassifiedContent
	require.NoError(t, json.Unmarshal(data, &got))

	require.NotNil(t, got.Indigenous)
	assert.Equal(t, "core_indigenous", got.Indigenous.Relevance)
	assert.Equal(t, []string{"culture", "language"}, got.Indigenous.Categories)
	assert.Equal(t, "ontario", got.Indigenous.Region)
}

// --- ClassificationResult tests ---

func TestClassificationResult_AllOptionalFieldsNil(t *testing.T) {
	t.Helper()

	result := domain.ClassificationResult{
		ContentID:   "doc-1",
		ContentType: domain.ContentTypeArticle,
	}

	assert.Equal(t, "doc-1", result.ContentID)
	assert.Equal(t, domain.ContentTypeArticle, result.ContentType)
	assert.Nil(t, result.Crime)
	assert.Nil(t, result.Mining)
	assert.Nil(t, result.Coforge)
	assert.Nil(t, result.Entertainment)
	assert.Nil(t, result.Indigenous)
	assert.Nil(t, result.Location)
	assert.Nil(t, result.Recipe)
	assert.Nil(t, result.Job)
	assert.Nil(t, result.RFP)
}

// --- ErrorCode tests ---

func TestErrorCodeValues(t *testing.T) {
	t.Helper()

	assert.Equal(t, domain.ErrorCodeESTimeout, domain.ErrorCode("ES_TIMEOUT"))
	assert.Equal(t, domain.ErrorCodeESUnavailable, domain.ErrorCode("ES_UNAVAILABLE"))
	assert.Equal(t, domain.ErrorCodeESIndexNotFound, domain.ErrorCode("ES_INDEX_NOT_FOUND"))
	assert.Equal(t, domain.ErrorCodeRulePanic, domain.ErrorCode("RULE_PANIC"))
	assert.Equal(t, domain.ErrorCodeQualityError, domain.ErrorCode("QUALITY_ERROR"))
	assert.Equal(t, domain.ErrorCodeContentType, domain.ErrorCode("CONTENT_TYPE_ERROR"))
	assert.Equal(t, domain.ErrorCodeIndexingFailed, domain.ErrorCode("INDEXING_FAILED"))
	assert.Equal(t, domain.ErrorCodeUnknown, domain.ErrorCode("UNKNOWN"))
}

// --- RecipeResult tests ---

func TestRecipeResult_JSONRoundTrip(t *testing.T) {
	t.Helper()

	prepTime := 15
	cookTime := 30
	totalTime := 45
	rating := 4.5
	ratingCount := 120

	recipe := domain.RecipeResult{
		ExtractionMethod: "schema_org",
		Name:             "Bannock",
		Ingredients:      []string{"flour", "baking powder", "salt", "water"},
		Instructions:     "Mix and fry",
		PrepTimeMinutes:  &prepTime,
		CookTimeMinutes:  &cookTime,
		TotalTimeMinutes: &totalTime,
		Servings:         "4",
		Category:         "bread",
		Cuisine:          "Indigenous",
		Rating:           &rating,
		RatingCount:      &ratingCount,
	}

	data, err := json.Marshal(recipe)
	require.NoError(t, err)

	var got domain.RecipeResult
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, recipe.Name, got.Name)
	assert.Equal(t, recipe.Ingredients, got.Ingredients)
	assert.Equal(t, *recipe.PrepTimeMinutes, *got.PrepTimeMinutes)
	assert.Equal(t, *recipe.CookTimeMinutes, *got.CookTimeMinutes)
	assert.InDelta(t, *recipe.Rating, *got.Rating, 0.01)
}

// --- JobResult tests ---

func TestJobResult_JSONRoundTrip(t *testing.T) {
	t.Helper()

	salaryMin := 50000.0
	salaryMax := 80000.0

	job := domain.JobResult{
		ExtractionMethod: "schema_org",
		Title:            "Software Developer",
		Company:          "North Cloud Inc",
		Location:         "Sudbury, ON",
		SalaryMin:        &salaryMin,
		SalaryMax:        &salaryMax,
		SalaryCurrency:   "CAD",
		EmploymentType:   "full_time",
	}

	data, err := json.Marshal(job)
	require.NoError(t, err)

	var got domain.JobResult
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, job.Title, got.Title)
	assert.Equal(t, job.Company, got.Company)
	assert.InDelta(t, *job.SalaryMin, *got.SalaryMin, 0.01)
	assert.InDelta(t, *job.SalaryMax, *got.SalaryMax, 0.01)
	assert.Equal(t, job.EmploymentType, got.EmploymentType)
}

// --- EntertainmentResult tests ---

func TestEntertainmentResult_JSONRoundTrip(t *testing.T) {
	t.Helper()

	ent := domain.EntertainmentResult{
		Relevance:        "core_entertainment",
		Categories:       []string{"film", "music"},
		FinalConfidence:  0.87,
		HomepageEligible: true,
		ReviewRequired:   false,
		ModelVersion:     "v1",
	}

	data, err := json.Marshal(ent)
	require.NoError(t, err)

	var got domain.EntertainmentResult
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, ent.Relevance, got.Relevance)
	assert.Equal(t, ent.Categories, got.Categories)
	assert.True(t, got.HomepageEligible)
}

// --- CoforgeResult tests ---

func TestCoforgeResult_JSONRoundTrip(t *testing.T) {
	t.Helper()

	cof := domain.CoforgeResult{
		Relevance:           "core_coforge",
		RelevanceConfidence: 0.91,
		Audience:            "developer",
		AudienceConfidence:  0.85,
		Topics:              []string{"cloud", "devops"},
		Industries:          []string{"technology"},
		FinalConfidence:     0.88,
	}

	data, err := json.Marshal(cof)
	require.NoError(t, err)

	var got domain.CoforgeResult
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, cof.Relevance, got.Relevance)
	assert.InDelta(t, cof.RelevanceConfidence, got.RelevanceConfidence, 0.001)
	assert.Equal(t, cof.Topics, got.Topics)
	assert.Equal(t, cof.Industries, got.Industries)
}

// --- ClassificationHistory tests ---

func TestClassificationHistory_JSONRoundTrip(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	history := domain.ClassificationHistory{
		ID:                    1,
		ContentID:             "doc-1",
		ContentURL:            "https://example.com/article",
		SourceName:            "example",
		ContentType:           domain.ContentTypeArticle,
		ContentSubtype:        domain.ContentSubtypeBlogPost,
		QualityScore:          75,
		Topics:                []string{"technology", "local_news"},
		SourceReputationScore: 80,
		ClassifierVersion:     "1.0.0",
		ClassificationMethod:  domain.MethodRuleBased,
		Confidence:            0.92,
		ProcessingTimeMs:      45,
		ClassifiedAt:          now,
	}

	data, err := json.Marshal(history)
	require.NoError(t, err)

	var got domain.ClassificationHistory
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, history.ContentID, got.ContentID)
	assert.Equal(t, history.ContentURL, got.ContentURL)
	assert.Equal(t, history.Topics, got.Topics)
	assert.Equal(t, history.ClassifierVersion, got.ClassifierVersion)
}

// --- MLModel tests ---

func TestMLModel_JSONRoundTrip(t *testing.T) {
	t.Helper()

	trainedAt := time.Now().UTC().Truncate(time.Second)
	model := domain.MLModel{
		ID:             1,
		ModelName:      "crime-classifier",
		ModelVersion:   "v2",
		ModelType:      "topic",
		Accuracy:       0.95,
		F1Score:        0.93,
		PrecisionScore: 0.94,
		RecallScore:    0.92,
		TrainedAt:      &trainedAt,
		FeatureSet:     []string{"title", "body", "url"},
		IsActive:       true,
		Enabled:        true,
	}

	data, err := json.Marshal(model)
	require.NoError(t, err)

	var got domain.MLModel
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, model.ModelName, got.ModelName)
	assert.Equal(t, model.ModelVersion, got.ModelVersion)
	assert.InDelta(t, model.Accuracy, got.Accuracy, 0.001)
	assert.Equal(t, model.FeatureSet, got.FeatureSet)
	assert.True(t, got.IsActive)
}

// --- DLQStats tests ---

func TestDLQStats_Fields(t *testing.T) {
	t.Helper()

	now := time.Now()
	stats := domain.DLQStats{
		Pending:     10,
		Exhausted:   2,
		Ready:       5,
		AvgRetries:  1.5,
		OldestEntry: &now,
	}

	data, err := json.Marshal(stats)
	require.NoError(t, err)

	var got domain.DLQStats
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, int64(10), got.Pending)
	assert.Equal(t, int64(2), got.Exhausted)
	assert.Equal(t, int64(5), got.Ready)
	assert.InDelta(t, 1.5, got.AvgRetries, 0.01)
}

func TestDLQSourceCount_Fields(t *testing.T) {
	t.Helper()

	sc := domain.DLQSourceCount{SourceName: "cbc", Count: 5}
	assert.Equal(t, "cbc", sc.SourceName)
	assert.Equal(t, int64(5), sc.Count)
}

func TestDLQErrorCount_Fields(t *testing.T) {
	t.Helper()

	ec := domain.DLQErrorCount{ErrorCode: domain.ErrorCodeESTimeout, Count: 3}
	assert.Equal(t, domain.ErrorCodeESTimeout, ec.ErrorCode)
	assert.Equal(t, int64(3), ec.Count)
}

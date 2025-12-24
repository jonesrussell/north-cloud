package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
)

// mockLogger implements Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}

// mockSourceReputationDB implements SourceReputationDB for testing
type mockSourceReputationDB struct {
	sources map[string]*domain.SourceReputation
}

func newMockSourceReputationDB() *mockSourceReputationDB {
	return &mockSourceReputationDB{
		sources: make(map[string]*domain.SourceReputation),
	}
}

func (m *mockSourceReputationDB) GetSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	source, ok := m.sources[sourceName]
	if !ok {
		return nil, nil
	}
	return source, nil
}

func (m *mockSourceReputationDB) CreateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) UpdateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) GetOrCreateSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	source, ok := m.sources[sourceName]
	if !ok {
		source = &domain.SourceReputation{
			SourceName:          sourceName,
			Category:            domain.SourceCategoryUnknown,
			ReputationScore:     50,
			TotalArticles:       0,
			AverageQualityScore: 0,
			SpamCount:           0,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}
		m.sources[sourceName] = source
	}
	return source, nil
}

// setupTestHandler creates a test handler with all dependencies
func setupTestHandler() *Handler {
	logger := &mockLogger{}

	// Create classification rules
	rules := []domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "charged", "suspect"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
	}

	// Create source reputation DB
	sourceRepDB := newMockSourceReputationDB()

	// Create classifier config
	config := classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: 50,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   0.25,
			MetadataWeight:    0.25,
			RichnessWeight:    0.25,
			ReadabilityWeight: 0.25,
			MinWordCount:      100,
			OptimalWordCount:  1000,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               50,
			UpdateOnEachClassification: true,
			SpamThreshold:              30,
			MinArticlesForTrust:        10,
			ReputationDecayRate:        0.1,
		},
	}

	// Create classifier and related components
	classifierInstance := classifier.NewClassifier(logger, rules, sourceRepDB, config)
	batchProcessor := processor.NewBatchProcessor(classifierInstance, 2, logger)
	sourceRepScorer := classifier.NewSourceReputationScorer(logger, sourceRepDB)
	topicClassifier := classifier.NewTopicClassifier(logger, rules)

	// For tests, pass nil for repositories as they're not used in most test cases
	// If a test needs them, it should create mock repositories
	return NewHandler(classifierInstance, batchProcessor, sourceRepScorer, topicClassifier, nil, nil, nil, logger)
}

// setupRouter creates a test router with routes
func setupRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router, handler)
	return router
}

func TestHealthCheck(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", response["status"])
	}
}

func TestReadyCheck(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "ready" {
		t.Errorf("expected status ready, got %v", response["status"])
	}
}

func TestClassify_Success(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	publishedDate := time.Now()
	reqBody := ClassifyRequest{
		RawContent: &domain.RawContent{
			ID:                   "test-1",
			URL:                  "https://example.com/article",
			SourceName:           "example.com",
			Title:                "Police arrest suspect in incident",
			RawText:              "Local police arrested a suspect yesterday following an incident downtown.",
			OGType:               "article",
			MetaDescription:      "Crime news",
			PublishedDate:        &publishedDate,
			ClassificationStatus: domain.StatusPending,
			WordCount:            200,
		},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/classify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ClassifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Result == nil {
		t.Fatal("expected result to be non-nil")
	}

	if response.Result.ContentType != domain.ContentTypeArticle {
		t.Errorf("expected content_type article, got %s", response.Result.ContentType)
	}

	// Verify crime is in topics array
	hasCrime := false
	for _, topic := range response.Result.Topics {
		if topic == "crime" {
			hasCrime = true
			break
		}
	}
	if !hasCrime {
		t.Error("expected crime to be in topics array")
	}
}

func TestClassify_InvalidRequest(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/classify", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestClassifyBatch_Success(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	publishedDate := time.Now()
	reqBody := BatchClassifyRequest{
		RawContents: []*domain.RawContent{
			{
				ID:                   "test-1",
				URL:                  "https://example.com/article1",
				SourceName:           "example.com",
				Title:                "Police arrest suspect",
				RawText:              "Local police arrested a suspect yesterday.",
				OGType:               "article",
				PublishedDate:        &publishedDate,
				ClassificationStatus: domain.StatusPending,
				WordCount:            200,
			},
			{
				ID:                   "test-2",
				URL:                  "https://example.com/article2",
				SourceName:           "example.com",
				Title:                "Sports team wins championship",
				RawText:              "The local team won the championship yesterday.",
				OGType:               "article",
				PublishedDate:        &publishedDate,
				ClassificationStatus: domain.StatusPending,
				WordCount:            200,
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/classify/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response BatchClassifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Total != 2 {
		t.Errorf("expected total 2, got %d", response.Total)
	}

	if response.Success != 2 {
		t.Errorf("expected success 2, got %d", response.Success)
	}

	if response.Failed != 0 {
		t.Errorf("expected failed 0, got %d", response.Failed)
	}
}

func TestClassifyBatch_EmptyRequest(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	reqBody := BatchClassifyRequest{
		RawContents: []*domain.RawContent{},
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/classify/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetSource(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/sources/example.com", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["source_name"] != "example.com" {
		t.Errorf("expected source_name example.com, got %v", response["source_name"])
	}

	// New source should have default score of 50
	if response["reputation_score"] != float64(50) {
		t.Errorf("expected reputation_score 50, got %v", response["reputation_score"])
	}
}

func TestGetSource_MissingName(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/sources/", nil)
	router.ServeHTTP(w, req)

	// Gin redirects /sources/ to /sources (301) which then returns 501 (not implemented)
	// Accept either redirect or not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusMovedPermanently && w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 404, 301, or 501, got %d", w.Code)
	}
}

func TestGetClassificationResult_NotImplemented(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/classify/test-123", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

func TestListRules_NotImplemented(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/rules", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

func TestCreateRule_NotImplemented(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	reqBody := CreateRuleRequest{
		Topic:    "test",
		Keywords: []string{"test"},
		Enabled:  true,
		Priority: "normal",
	}

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

func TestListSources_NotImplemented(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/sources", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

func TestGetStats_NotImplemented(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

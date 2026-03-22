//nolint:testpackage // Testing internal API handlers requires same package access
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/testhelpers"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// mockLogger implements Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Info(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Warn(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Error(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Fatal(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) With(fields ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                         { return nil }

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
	sourceRepDB := testhelpers.NewMockSourceReputationDB()

	// Create classifier config
	classifierCfg := classifier.Config{
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
	classifierInstance := classifier.NewClassifier(logger, rules, sourceRepDB, classifierCfg)
	batchProcessor := processor.NewBatchProcessor(classifierInstance, 2, logger)
	sourceRepScorer := classifier.NewSourceReputationScorer(logger, sourceRepDB)
	topicClassifier := classifier.NewTopicClassifier(logger, rules, 5)

	testCfg := &config.Config{}
	return NewHandler(classifierInstance, batchProcessor, sourceRepScorer, topicClassifier, nil, sourceRepDB, nil, nil, testCfg, logger)
}

// setupRouter creates a test router with routes
func setupRouter(handler *Handler) *gin.Engine {
	// Create test config without JWT secret to disable JWT authentication in tests
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "", // No JWT secret for tests
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRoutes(router, handler, cfg)
	return router
}

func TestHealthCheck(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
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
	req, _ := http.NewRequest(http.MethodGet, "/ready", http.NoBody)
	router.ServeHTTP(w, req)

	// With nil dependencies, we expect ready status (unconfigured is not unhealthy)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["status"] != "ready" {
		t.Errorf("expected status ready, got %v", response["status"])
	}

	// Verify checks field exists
	checks, ok := response["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected checks to be a map")
	}

	// With test setup, postgresql and elasticsearch are unconfigured
	if checks["postgresql"] != "unconfigured" {
		t.Logf("postgresql check: %v", checks["postgresql"])
	}
	if checks["redis"] != "not_applicable" {
		t.Errorf("expected redis to be not_applicable, got %v", checks["redis"])
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

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ClassifyResponse
	if unmarshalErr := json.Unmarshal(w.Body.Bytes(), &response); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal response: %v", unmarshalErr)
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
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify", bytes.NewBufferString("{}"))
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

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response BatchClassifyResponse
	if unmarshalErr := json.Unmarshal(w.Body.Bytes(), &response); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal response: %v", unmarshalErr)
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

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify/batch", bytes.NewBuffer(body))
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
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sources/example.com", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["name"] != "example.com" {
		t.Errorf("expected name example.com, got %v", response["name"])
	}

	// New source should have default score of 50
	if response["reputation"] != float64(50) {
		t.Errorf("expected reputation 50, got %v", response["reputation"])
	}
}

func TestGetSource_MissingName(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/sources/", http.NoBody)
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
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/classify/test-123", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}

func TestTestRule_InvalidRuleID(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	reqBody := TestRuleRequest{
		Title: "Test",
		Body:  "Test body content",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/rules/invalid/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTestRule_MissingBody(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	// Body field is required - empty body should fail validation
	reqBody := TestRuleRequest{
		Title: "Test Title",
		Body:  "", // Empty body should fail required validation
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/rules/1/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestTestRule_RepoNotConfigured(t *testing.T) {
	handler := setupTestHandler()
	router := setupRouter(handler)

	reqBody := TestRuleRequest{
		Title: "Test",
		Body:  "Test body content that is valid",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/rules/999/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Returns 503 because rulesRepo is nil in test handler
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d: %s", w.Code, w.Body.String())
	}
}

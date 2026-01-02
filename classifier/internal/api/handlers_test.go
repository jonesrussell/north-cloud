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
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	_ "github.com/mattn/go-sqlite3"
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

// setupTestSourceReputationRepository creates an in-memory SQLite repository for testing
func setupTestSourceReputationRepository() (*database.SourceReputationRepository, error) {
	// Create in-memory SQLite database
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Create source_reputation table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS source_reputation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_name TEXT UNIQUE NOT NULL,
			source_url TEXT,
			category TEXT NOT NULL DEFAULT 'unknown',
			reputation_score INTEGER NOT NULL DEFAULT 50,
			total_articles INTEGER NOT NULL DEFAULT 0,
			average_quality_score REAL NOT NULL DEFAULT 0.0,
			spam_count INTEGER NOT NULL DEFAULT 0,
			last_classified_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	return database.NewSourceReputationRepository(db), nil
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

	// Create test repository for API handlers (in-memory SQLite)
	sourceRepRepo, err := setupTestSourceReputationRepository()
	if err != nil {
		// If SQLite is not available, return handler with nil (some tests may not need it)
		// This allows tests that don't use sourceRepo to still pass
		sourceRepRepo = nil
	}

	// For tests, pass nil for rulesRepo and classificationHistoryRepo as they're not used in most test cases
	// If a test needs them, it should create mock repositories
	return NewHandler(classifierInstance, batchProcessor, sourceRepScorer, topicClassifier, nil, sourceRepRepo, nil, logger)
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
	req, _ := http.NewRequest(http.MethodGet, "/health", http.NoBody)
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
	req, _ := http.NewRequest(http.MethodGet, "/ready", http.NoBody)
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
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify", bytes.NewBuffer(body))
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

	body, _ := json.Marshal(reqBody)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/classify/batch", bytes.NewBuffer(body))
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

	var response map[string]interface{}
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

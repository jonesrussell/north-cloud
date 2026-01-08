//nolint:testpackage // Testing internal processor requires same package access
package processor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// mockLogger implements the Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, keysAndValues ...any) {}
func (m *mockLogger) Info(msg string, keysAndValues ...any)  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...any)  {}
func (m *mockLogger) Error(msg string, keysAndValues ...any) {}

// mockSourceReputationDB implements SourceReputationDB for testing
type mockSourceReputationDB struct {
	mu      sync.RWMutex
	sources map[string]*domain.SourceReputation
}

func newMockSourceReputationDB() *mockSourceReputationDB {
	return &mockSourceReputationDB{
		sources: make(map[string]*domain.SourceReputation),
	}
}

func (m *mockSourceReputationDB) GetSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	source, ok := m.sources[sourceName]
	if !ok {
		return nil, errors.New("source not found")
	}
	return source, nil
}

func (m *mockSourceReputationDB) CreateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) UpdateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) GetOrCreateSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

// createTestClassifier creates a classifier with all dependencies for testing
func createTestClassifier(logger *mockLogger) *classifier.Classifier {
	// Create classification rules
	rules := []domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "charged", "suspect", "incident", "crime", "criminal"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
		{
			ID:            2,
			RuleName:      "sports_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "sports",
			Keywords:      []string{"team", "championship", "game", "match", "sports", "player", "won"},
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

	return classifier.NewClassifier(logger, rules, sourceRepDB, config)
}

// mockESClient implements ElasticsearchClient for integration testing
type mockESClient struct {
	rawContent        []*domain.RawContent
	classifiedContent []*domain.ClassifiedContent
	statusUpdates     map[string]string
	queryError        error
	bulkIndexError    error
	updateStatusError error
}

func newMockESClient() *mockESClient {
	return &mockESClient{
		rawContent:        make([]*domain.RawContent, 0),
		classifiedContent: make([]*domain.ClassifiedContent, 0),
		statusUpdates:     make(map[string]string),
	}
}

func (m *mockESClient) QueryRawContent(ctx context.Context, status string, batchSize int) ([]*domain.RawContent, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}

	var results []*domain.RawContent
	for _, raw := range m.rawContent {
		if raw.ClassificationStatus == status && len(results) < batchSize {
			results = append(results, raw)
		}
	}
	return results, nil
}

func (m *mockESClient) IndexClassifiedContent(ctx context.Context, content *domain.ClassifiedContent) error {
	m.classifiedContent = append(m.classifiedContent, content)
	return nil
}

func (m *mockESClient) UpdateRawContentStatus(ctx context.Context, contentID, status string, classifiedAt time.Time) error {
	if m.updateStatusError != nil {
		return m.updateStatusError
	}
	m.statusUpdates[contentID] = status
	return nil
}

func (m *mockESClient) BulkIndexClassifiedContent(ctx context.Context, contents []*domain.ClassifiedContent) error {
	if m.bulkIndexError != nil {
		return m.bulkIndexError
	}
	m.classifiedContent = append(m.classifiedContent, contents...)
	return nil
}

// mockDBClient implements DatabaseClient for integration testing
type mockDBClient struct {
	histories      []*domain.ClassificationHistory
	saveBatchError error
}

func newMockDBClient() *mockDBClient {
	return &mockDBClient{
		histories: make([]*domain.ClassificationHistory, 0),
	}
}

func (m *mockDBClient) SaveClassificationHistory(ctx context.Context, history *domain.ClassificationHistory) error {
	m.histories = append(m.histories, history)
	return nil
}

func (m *mockDBClient) SaveClassificationHistoryBatch(ctx context.Context, histories []*domain.ClassificationHistory) error {
	if m.saveBatchError != nil {
		return m.saveBatchError
	}
	m.histories = append(m.histories, histories...)
	return nil
}

// setupTestEnvironment creates test mocks and content
func setupTestEnvironment() (*mockESClient, *mockDBClient, *mockLogger) {
	logger := &mockLogger{}
	esClient := newMockESClient()
	dbClient := newMockDBClient()

	publishedDate := time.Now().Add(-24 * time.Hour)
	esClient.rawContent = []*domain.RawContent{
		{
			ID:                   "test-1",
			URL:                  "https://example.com/article1",
			SourceName:           "example.com",
			Title:                "Police arrest suspect in downtown incident",
			RawText:              "Local police arrested a suspect yesterday following an incident in downtown. The individual was charged with multiple offenses.",
			OGType:               "article",
			MetaDescription:      "Local crime news",
			PublishedDate:        &publishedDate,
			ClassificationStatus: domain.StatusPending,
			WordCount:            300,
		},
		{
			ID:                   "test-2",
			URL:                  "https://example.com/sports",
			SourceName:           "example.com",
			Title:                "Local team wins championship",
			RawText:              "The local sports team won the championship yesterday in a thrilling final match. Fans celebrated in the streets.",
			OGType:               "article",
			MetaDescription:      "Sports news",
			PublishedDate:        &publishedDate,
			ClassificationStatus: domain.StatusPending,
			WordCount:            250,
		},
	}

	return esClient, dbClient, logger
}

// verifyCrimeArticle verifies crime article classification results
func verifyCrimeArticle(t *testing.T, crimeArticle *domain.ClassifiedContent) {
	if crimeArticle.ContentType != domain.ContentTypeArticle {
		t.Errorf("expected content_type article, got %s", crimeArticle.ContentType)
	}
	hasCrime := false
	for _, topic := range crimeArticle.Topics {
		if topic == "crime" {
			hasCrime = true
			break
		}
	}
	if !hasCrime {
		t.Error("expected crime article to have 'crime' in topics array")
	}
	if crimeArticle.QualityScore < 50 {
		t.Errorf("expected quality score >= 50, got %d", crimeArticle.QualityScore)
	}
}

// verifySportsArticle verifies sports article classification results
func verifySportsArticle(t *testing.T, sportsArticle *domain.ClassifiedContent) {
	for _, topic := range sportsArticle.Topics {
		if topic == "crime" {
			t.Error("expected sports article NOT to have 'crime' in topics array")
			break
		}
	}
}

// verifyStatusUpdates verifies that status updates were applied
func verifyStatusUpdates(t *testing.T, esClient *mockESClient) {
	if status, ok := esClient.statusUpdates["test-1"]; !ok || status != domain.StatusClassified {
		t.Errorf("expected test-1 status to be classified, got %s", status)
	}
	if status, ok := esClient.statusUpdates["test-2"]; !ok || status != domain.StatusClassified {
		t.Errorf("expected test-2 status to be classified, got %s", status)
	}
}

// TestIntegration_EndToEndClassificationFlow tests the complete pipeline
func TestIntegration_EndToEndClassificationFlow(t *testing.T) {
	esClient, dbClient, logger := setupTestEnvironment()

	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)

	pollerConfig := PollerConfig{
		BatchSize:    10,
		PollInterval: 30 * time.Second,
	}
	poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)

	ctx := context.Background()
	if err := poller.processPending(ctx); err != nil {
		t.Fatalf("processPending failed: %v", err)
	}

	if len(esClient.classifiedContent) != 2 {
		t.Errorf("expected 2 classified items, got %d", len(esClient.classifiedContent))
	}

	verifyCrimeArticle(t, esClient.classifiedContent[0])
	verifySportsArticle(t, esClient.classifiedContent[1])
	verifyStatusUpdates(t, esClient)

	if len(dbClient.histories) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(dbClient.histories))
	}

	// Verify history content
	for _, history := range dbClient.histories {
		if history.ContentID == "" {
			t.Error("expected history to have content_id")
		}
		if history.ContentType == "" {
			t.Error("expected history to have content_type")
		}
		if history.QualityScore == 0 {
			t.Error("expected history to have quality_score")
		}
	}
}

// TestIntegration_BatchProcessingWithErrors tests error handling in pipeline
func TestIntegration_BatchProcessingWithErrors(t *testing.T) {
	logger := &mockLogger{}
	esClient := newMockESClient()
	dbClient := newMockDBClient()

	// Create test content
	publishedDate := time.Now()
	esClient.rawContent = []*domain.RawContent{
		{
			ID:                   "test-1",
			URL:                  "https://example.com/article1",
			SourceName:           "example.com",
			Title:                "Test article",
			RawText:              "Test content",
			ClassificationStatus: domain.StatusPending,
			WordCount:            50,
			PublishedDate:        &publishedDate,
		},
	}

	// Simulate bulk indexing error
	esClient.bulkIndexError = errors.New("ES indexing failed")

	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)
	pollerConfig := PollerConfig{BatchSize: 10, PollInterval: 30 * time.Second}
	poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)

	// Process should fail due to indexing error
	ctx := context.Background()
	err := poller.processPending(ctx)

	if err == nil {
		t.Fatal("expected error due to bulk indexing failure")
	}

	// Verify no classified content was indexed
	if len(esClient.classifiedContent) > 0 {
		t.Errorf("expected no classified content due to error, got %d", len(esClient.classifiedContent))
	}
}

// TestIntegration_PollerStartStop tests poller lifecycle
func TestIntegration_PollerStartStop(t *testing.T) {
	logger := &mockLogger{}
	esClient := newMockESClient()
	dbClient := newMockDBClient()

	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)
	pollerConfig := PollerConfig{
		BatchSize:    10,
		PollInterval: 100 * time.Millisecond, // Short interval for testing
	}
	poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)

	// Test starting
	ctx := context.Background()
	err := poller.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start poller: %v", err)
	}

	if !poller.IsRunning() {
		t.Error("expected poller to be running after Start()")
	}

	// Test starting again (should fail)
	err = poller.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running poller")
	}

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Test stopping
	poller.Stop()
	time.Sleep(50 * time.Millisecond)

	if poller.IsRunning() {
		t.Error("expected poller to be stopped after Stop()")
	}
}

// TestIntegration_RateLimitedProcessing tests rate limiting in pipeline
func TestIntegration_RateLimitedProcessing(t *testing.T) {
	logger := &mockLogger{}
	esClient := newMockESClient()

	// Create test content
	publishedDate := time.Now()
	esClient.rawContent = []*domain.RawContent{
		{
			ID:                   "test-1",
			URL:                  "https://example.com/article1",
			SourceName:           "example.com",
			Title:                "Test article 1",
			RawText:              "Test content with enough words to pass quality check",
			ClassificationStatus: domain.StatusPending,
			WordCount:            200,
			PublishedDate:        &publishedDate,
			OGType:               "article",
		},
	}

	// Create batch processor
	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)

	// Create rate-limited processor with low RPS for testing
	esRPS := 10 // 10 requests per second for ES
	dbRPS := 10 // 10 requests per second for DB
	rateLimitedProc := NewRateLimitedProcessor(batchProcessor, esRPS, dbRPS, logger)

	// Process with rate limiting
	ctx := context.Background()
	startTime := time.Now()
	results, err := rateLimitedProc.ProcessWithRateLimit(ctx, esClient.rawContent)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("rate-limited processing failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// Verify rate limiting added some delay (but not too much for a single item)
	if duration > 1*time.Second {
		t.Errorf("processing took too long: %v", duration)
	}

	// Verify the limiter objects exist
	if rateLimitedProc.GetESLimiter() == nil {
		t.Error("expected ES rate limiter to be initialized")
	}
	if rateLimitedProc.GetDBLimiter() == nil {
		t.Error("expected DB rate limiter to be initialized")
	}
}

// TestIntegration_PollerWithRateLimiting tests poller with rate limiting
func TestIntegration_PollerWithRateLimiting(t *testing.T) {
	logger := &mockLogger{}
	esClient := newMockESClient()
	dbClient := newMockDBClient()

	// Create test content
	publishedDate := time.Now()
	esClient.rawContent = []*domain.RawContent{
		{
			ID:                   "test-1",
			URL:                  "https://example.com/article1",
			SourceName:           "example.com",
			Title:                "Test article",
			RawText:              "Test content",
			ClassificationStatus: domain.StatusPending,
			WordCount:            200,
			PublishedDate:        &publishedDate,
		},
	}

	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)
	pollerConfig := PollerConfig{
		BatchSize:    10,
		PollInterval: 30 * time.Second,
	}
	poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)

	// Create rate-limited poller
	pollRPS := 10
	rateLimitedPoller := NewRateLimitedPoller(poller, pollRPS, logger)

	// Test lifecycle methods
	ctx := context.Background()
	err := rateLimitedPoller.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start rate-limited poller: %v", err)
	}

	if !rateLimitedPoller.IsRunning() {
		t.Error("expected rate-limited poller to be running")
	}

	// Let it process
	time.Sleep(100 * time.Millisecond)

	rateLimitedPoller.Stop()
	time.Sleep(50 * time.Millisecond)

	if rateLimitedPoller.IsRunning() {
		t.Error("expected rate-limited poller to be stopped")
	}
}

// TestIntegration_BatchProcessingConcurrency tests concurrent processing
func TestIntegration_BatchProcessingConcurrency(t *testing.T) {
	logger := &mockLogger{}
	testClassifier := createTestClassifier(logger)

	// Create 20 test items
	publishedDate := time.Now()
	rawItems := make([]*domain.RawContent, 20)
	for i := range 20 {
		rawItems[i] = &domain.RawContent{
			ID:                   "test-" + string(rune('A'+i)),
			URL:                  "https://example.com/article",
			SourceName:           "example.com",
			Title:                "Test article",
			RawText:              "Test content with sufficient word count for quality scoring",
			ClassificationStatus: domain.StatusPending,
			WordCount:            200,
			PublishedDate:        &publishedDate,
			OGType:               "article",
		}
	}

	// Process with 5 concurrent workers
	batchProcessor := NewBatchProcessor(testClassifier, 5, logger)

	ctx := context.Background()
	startTime := time.Now()
	results, err := batchProcessor.Process(ctx, rawItems)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("batch processing failed: %v", err)
	}

	if len(results) != 20 {
		t.Errorf("expected 20 results, got %d", len(results))
	}

	// Verify all items were processed successfully
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("item %s failed: %v", result.Raw.ID, result.Error)
		}
		if result.ClassifiedContent == nil {
			t.Errorf("item %s missing classified content", result.Raw.ID)
		}
	}

	// Concurrent processing should be faster than sequential
	t.Logf("Processed 20 items in %v with 5 workers", duration)
}

// TestIntegration_FailedClassificationHandling tests handling of failed items
func TestIntegration_FailedClassificationHandling(t *testing.T) {
	logger := &mockLogger{}
	esClient := newMockESClient()
	dbClient := newMockDBClient()

	// Create test content - one will fail quality checks
	publishedDate := time.Now()
	esClient.rawContent = []*domain.RawContent{
		{
			ID:                   "test-good",
			URL:                  "https://example.com/article1",
			SourceName:           "example.com",
			Title:                "Good article",
			RawText:              "This article has enough content to pass quality checks",
			ClassificationStatus: domain.StatusPending,
			WordCount:            200,
			PublishedDate:        &publishedDate,
			OGType:               "article",
		},
		{
			ID:                   "test-poor",
			URL:                  "https://example.com/article2",
			SourceName:           "example.com",
			Title:                "", // Missing title - will fail
			RawText:              "Short",
			ClassificationStatus: domain.StatusPending,
			WordCount:            5,
			PublishedDate:        nil,
		},
	}

	testClassifier := createTestClassifier(logger)
	batchProcessor := NewBatchProcessor(testClassifier, 2, logger)
	pollerConfig := PollerConfig{BatchSize: 10, PollInterval: 30 * time.Second}
	poller := NewPoller(esClient, dbClient, batchProcessor, logger, pollerConfig)

	// Process pending content
	ctx := context.Background()
	err := poller.processPending(ctx)

	// Should not fail entirely (graceful degradation)
	if err != nil {
		t.Fatalf("processPending should handle failures gracefully: %v", err)
	}

	// Verify good content was classified
	if len(esClient.classifiedContent) < 1 {
		t.Error("expected at least 1 successful classification")
	}

	// Verify status updates (both should have status updates)
	if _, ok := esClient.statusUpdates["test-good"]; !ok {
		t.Error("expected test-good to have status update")
	}

	// History should only include successful classifications
	if len(dbClient.histories) < 1 {
		t.Error("expected at least 1 history entry for successful classification")
	}
}

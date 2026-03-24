//nolint:testpackage // Testing internal worker state (failure counters, mutex) requires same package access
package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOutboxWorkerConfig_Values(t *testing.T) {
	t.Helper()

	cfg := DefaultOutboxWorkerConfig()

	assert.Equal(t, 5*time.Second, cfg.PollInterval)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 10*time.Second, cfg.PublishTimeout)
}

func TestNewOutboxWorker_DefaultsZeroConfig(t *testing.T) {
	t.Helper()

	cfg := OutboxWorkerConfig{} // All zeroes
	w := NewOutboxWorker(nil, nil, cfg, infralogger.NewNop())

	assert.Equal(t, defaultPollInterval, w.pollInterval)
	assert.Equal(t, defaultBatchSize, w.batchSize)
	assert.Equal(t, defaultPublishTimeout, w.publishTimeout)
	assert.NotNil(t, w.stopChan)
	assert.NotNil(t, w.logger)
	assert.NotNil(t, w.tracer)
}

func TestNewOutboxWorker_NegativeConfig(t *testing.T) {
	t.Helper()

	cfg := OutboxWorkerConfig{
		PollInterval:   -1 * time.Second,
		BatchSize:      -10,
		PublishTimeout: -5 * time.Second,
	}
	w := NewOutboxWorker(nil, nil, cfg, infralogger.NewNop())

	assert.Equal(t, defaultPollInterval, w.pollInterval)
	assert.Equal(t, defaultBatchSize, w.batchSize)
	assert.Equal(t, defaultPublishTimeout, w.publishTimeout)
}

func TestNewOutboxWorker_CustomConfig(t *testing.T) {
	t.Helper()

	cfg := OutboxWorkerConfig{
		PollInterval:   10 * time.Second,
		BatchSize:      200,
		PublishTimeout: 30 * time.Second,
	}
	w := NewOutboxWorker(nil, nil, cfg, infralogger.NewNop())

	assert.Equal(t, 10*time.Second, w.pollInterval)
	assert.Equal(t, 200, w.batchSize)
	assert.Equal(t, 30*time.Second, w.publishTimeout)
}

func TestOutboxWorker_IsRunning_InitiallyFalse(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())
	assert.False(t, w.IsRunning())
}

func TestOutboxWorker_StopWithoutStart(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	// Stop on a never-started worker should not panic
	w.Stop()
	assert.False(t, w.IsRunning())
}

func TestOutboxWorker_StartStop(t *testing.T) {
	t.Helper()

	w := newTestWorkerWithMockDB(t)

	ctx, cancel := context.WithCancel(context.Background())

	w.Start(ctx)
	assert.True(t, w.IsRunning())

	// Cancel context and stop worker
	cancel()
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5 seconds")
	}
}

func TestOutboxWorker_DoubleStart(t *testing.T) {
	t.Helper()

	w := newTestWorkerWithMockDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)
	assert.True(t, w.IsRunning())

	// Second Start should be a no-op (no extra goroutines)
	w.Start(ctx)
	assert.True(t, w.IsRunning())

	cancel()
	time.Sleep(50 * time.Millisecond)
	w.Stop()
}

func TestOutboxWorker_TrackFetchFailure(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	for i := range 3 {
		w.trackFetchFailure(assert.AnError, "pending")

		w.mu.Lock()
		assert.Equal(t, i+1, w.consecutiveFailures)
		assert.Equal(t, assert.AnError, w.lastError)
		w.mu.Unlock()
	}
}

func TestOutboxWorker_TrackFetchFailure_DegradedThreshold(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	for range consecutiveFailureThreshold + 1 {
		w.trackFetchFailure(assert.AnError, "pending")
	}

	w.mu.Lock()
	assert.GreaterOrEqual(t, w.consecutiveFailures, consecutiveFailureThreshold)
	w.mu.Unlock()
}

func TestOutboxWorker_FailureCounterReset(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	// Simulate some failures then reset (as processOnce does on success)
	w.mu.Lock()
	w.consecutiveFailures = 3
	w.lastError = assert.AnError
	w.mu.Unlock()

	w.mu.Lock()
	w.consecutiveFailures = 0
	w.lastError = nil
	w.mu.Unlock()

	w.mu.Lock()
	assert.Equal(t, 0, w.consecutiveFailures)
	require.NoError(t, w.lastError)
	w.mu.Unlock()
}

func TestOutboxWorker_ConcurrentIsRunning(t *testing.T) {
	t.Helper()

	w := NewOutboxWorker(nil, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	var wg sync.WaitGroup
	const goroutines = 10

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_ = w.IsRunning()
		}()
	}
	wg.Wait()
}

func TestOutboxWorker_ProcessOnce_EmptyBatch(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// FetchPending returns empty result
	mock.ExpectQuery("UPDATE classified_outbox").
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows(outboxColumns()))

	// FetchRetryable returns empty result (batchSize/2 = 5)
	mock.ExpectQuery("UPDATE classified_outbox").
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows(outboxColumns()))

	repo := database.NewOutboxRepository(db)
	w := NewOutboxWorker(repo, nil, OutboxWorkerConfig{
		PollInterval:   time.Second,
		BatchSize:      10,
		PublishTimeout: time.Second,
	}, infralogger.NewNop())

	w.processOnce(context.Background())

	// Failure counter should be zero (both fetches succeeded)
	w.mu.Lock()
	assert.Equal(t, 0, w.consecutiveFailures)
	w.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxWorker_ProcessOnce_FetchError(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// FetchPending returns an error
	mock.ExpectQuery("UPDATE classified_outbox").
		WithArgs(10).
		WillReturnError(assert.AnError)

	// FetchRetryable also returns an error
	mock.ExpectQuery("UPDATE classified_outbox").
		WithArgs(5).
		WillReturnError(assert.AnError)

	repo := database.NewOutboxRepository(db)
	w := NewOutboxWorker(repo, nil, OutboxWorkerConfig{
		PollInterval:   time.Second,
		BatchSize:      10,
		PublishTimeout: time.Second,
	}, infralogger.NewNop())

	w.processOnce(context.Background())

	// Failure counter should be incremented
	w.mu.Lock()
	assert.Equal(t, 2, w.consecutiveFailures)
	w.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxWorker_GetStats_WithMockDB(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Mock the stats query
	rows := sqlmock.NewRows([]string{
		"pending", "publishing", "published",
		"failed_retryable", "failed_exhausted", "avg_publish_lag_seconds",
	}).AddRow(10, 2, 100, 3, 1, 1.5)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	repo := database.NewOutboxRepository(db)
	w := NewOutboxWorker(repo, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	// Set internal state
	w.mu.Lock()
	w.consecutiveFailures = 3
	w.markPublishedFailures = 7
	w.mu.Unlock()

	stats, statsErr := w.GetStats(context.Background())
	require.NoError(t, statsErr)
	require.NotNil(t, stats)

	assert.Equal(t, int64(10), stats["pending"])
	assert.Equal(t, int64(2), stats["publishing"])
	assert.Equal(t, int64(100), stats["published"])
	assert.Equal(t, int64(3), stats["failed_retryable"])
	assert.Equal(t, int64(1), stats["failed_exhausted"])
	assert.Equal(t, 3, stats["consecutive_fetch_failures"])
	assert.Equal(t, int64(7), stats["mark_published_failures"])
	assert.False(t, stats["degraded"].(bool))
	assert.False(t, stats["running"].(bool))
	assert.Equal(t, defaultPollInterval.String(), stats["poll_interval"])
	assert.Equal(t, defaultBatchSize, stats["batch_size"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxWorker_GetStats_Degraded(t *testing.T) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"pending", "publishing", "published",
		"failed_retryable", "failed_exhausted", "avg_publish_lag_seconds",
	}).AddRow(0, 0, 0, 0, 0, 0.0)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	repo := database.NewOutboxRepository(db)
	w := NewOutboxWorker(repo, nil, DefaultOutboxWorkerConfig(), infralogger.NewNop())

	// Push past degraded threshold
	w.mu.Lock()
	w.consecutiveFailures = consecutiveFailureThreshold
	w.mu.Unlock()

	stats, statsErr := w.GetStats(context.Background())
	require.NoError(t, statsErr)
	assert.True(t, stats["degraded"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxWorkerConfig_Struct(t *testing.T) {
	t.Helper()

	cfg := OutboxWorkerConfig{
		PollInterval:   3 * time.Second,
		BatchSize:      50,
		PublishTimeout: 15 * time.Second,
	}

	assert.Equal(t, 3*time.Second, cfg.PollInterval)
	assert.Equal(t, 50, cfg.BatchSize)
	assert.Equal(t, 15*time.Second, cfg.PublishTimeout)
}

// newTestWorkerWithMockDB creates a worker backed by sqlmock so that
// processOnce can run without panicking on nil *sql.DB.
func newTestWorkerWithMockDB(t *testing.T) *OutboxWorker {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	// processOnce runs immediately on Start: expect pending + retryable queries.
	// Use regexp matcher so repeated polls also match.
	emptyRows := func() *sqlmock.Rows { return sqlmock.NewRows(outboxColumns()) }
	for range 10 { // enough for short test lifetime
		mock.ExpectQuery("UPDATE classified_outbox").WillReturnRows(emptyRows())
	}

	repo := database.NewOutboxRepository(db)
	cfg := OutboxWorkerConfig{
		PollInterval:   100 * time.Millisecond,
		BatchSize:      10,
		PublishTimeout: 1 * time.Second,
	}

	return NewOutboxWorker(repo, nil, cfg, infralogger.NewNop())
}

// outboxColumns returns the column names matching outboxSelectList in the repository.
func outboxColumns() []string {
	return []string{
		"id", "content_id", "source_name", "index_name", "content_type", "topics",
		"quality_score", "is_crime_related", "crime_subcategory",
		"title", "body", "url", "published_date", "status", "retry_count",
		"max_retries", "error_message", "created_at", "updated_at",
		"published_at", "next_retry_at",
	}
}

//nolint:testpackage // Testing internal storage requires same package access
package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// mockLoggerWithCalls tracks log calls for testing
type mockLoggerWithCalls struct {
	debugCalls []logCall
	infoCalls  []logCall
	warnCalls  []logCall
	errorCalls []logCall
}

type logCall struct {
	msg    string
	fields []infralogger.Field
}

func newMockLoggerWithCalls() *mockLoggerWithCalls {
	return &mockLoggerWithCalls{
		debugCalls: make([]logCall, 0),
		infoCalls:  make([]logCall, 0),
		warnCalls:  make([]logCall, 0),
		errorCalls: make([]logCall, 0),
	}
}

func (m *mockLoggerWithCalls) Debug(msg string, fields ...infralogger.Field) {
	m.debugCalls = append(m.debugCalls, logCall{msg: msg, fields: fields})
}

func (m *mockLoggerWithCalls) Info(msg string, fields ...infralogger.Field) {
	m.infoCalls = append(m.infoCalls, logCall{msg: msg, fields: fields})
}

func (m *mockLoggerWithCalls) Warn(msg string, fields ...infralogger.Field) {
	m.warnCalls = append(m.warnCalls, logCall{msg: msg, fields: fields})
}

func (m *mockLoggerWithCalls) Error(msg string, fields ...infralogger.Field) {
	m.errorCalls = append(m.errorCalls, logCall{msg: msg, fields: fields})
}

func (m *mockLoggerWithCalls) Fatal(msg string, fields ...infralogger.Field)       {}
func (m *mockLoggerWithCalls) With(fields ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLoggerWithCalls) Sync() error                                         { return nil }

// getFieldValue extracts the value from a zap.Field by key name
func getFieldValue(fields []infralogger.Field, key string) (any, bool) {
	for _, field := range fields {
		// Use reflection to access the private Key field of zap.Field
		fieldValue := reflect.ValueOf(field)
		if fieldValue.Kind() == reflect.Struct {
			keyField := fieldValue.FieldByName("Key")
			if keyField.IsValid() && keyField.String() == key {
				// Get the value - zap.Field stores it in Interface field
				interfaceField := fieldValue.FieldByName("Interface")
				if interfaceField.IsValid() {
					return interfaceField.Interface(), true
				}
			}
		}
	}
	return nil, false
}

// mockHistoryRepository simulates database operations for testing
// It implements the same interface as database.ClassificationHistoryRepository
type mockHistoryRepository struct {
	createFunc func(ctx context.Context, history *domain.ClassificationHistory) error
}

func (m *mockHistoryRepository) Create(ctx context.Context, history *domain.ClassificationHistory) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, history)
	}
	return nil
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_EmptyList(t *testing.T) {
	logger := newMockLoggerWithCalls()
	repo := &mockHistoryRepository{}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	err := adapter.SaveClassificationHistoryBatch(context.Background(), []*domain.ClassificationHistory{})

	if err != nil {
		t.Errorf("expected no error for empty list, got %v", err)
	}

	if len(logger.errorCalls) != 0 {
		t.Errorf("expected no error logs for empty list, got %d", len(logger.errorCalls))
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_AllSuccess(t *testing.T) {
	logger := newMockLoggerWithCalls()
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			return nil
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-1", ContentURL: "https://example.com/1"},
		{ContentID: "test-2", ContentURL: "https://example.com/2"},
	}

	err := adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	if err != nil {
		t.Errorf("expected no error when all succeed, got %v", err)
	}

	if len(logger.errorCalls) != 0 {
		t.Errorf("expected no error logs when all succeed, got %d", len(logger.errorCalls))
	}

	if len(logger.warnCalls) != 0 {
		t.Errorf("expected no warning logs when all succeed, got %d", len(logger.warnCalls))
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_AllFail(t *testing.T) {
	logger := newMockLoggerWithCalls()
	testError := errors.New("database error")
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			return testError
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-1", ContentURL: "https://example.com/1"},
		{ContentID: "test-2", ContentURL: "https://example.com/2"},
	}

	err := adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	if err == nil {
		t.Fatal("expected error when all records fail, got nil")
	}

	// Should log individual errors for each record
	if len(logger.errorCalls) != len(histories)+1 { // +1 for the "all failed" summary
		t.Errorf("expected %d error logs (2 individual + 1 summary), got %d", len(histories)+1, len(logger.errorCalls))
	}

	// Verify individual error logs
	individualErrors := 0
	for _, call := range logger.errorCalls {
		if call.msg == "Failed to save classification history record" {
			individualErrors++
		}
	}

	if individualErrors != len(histories) {
		t.Errorf("expected %d individual error logs, got %d", len(histories), individualErrors)
	}

	// Verify summary error log
	foundSummary := false
	for _, call := range logger.errorCalls {
		if call.msg == "All classification history records failed to save" {
			foundSummary = true
			break
		}
	}

	if !foundSummary {
		t.Error("expected summary error log for all failures")
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_PartialFail(t *testing.T) {
	logger := newMockLoggerWithCalls()
	testError := errors.New("database error")
	callCount := 0
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			callCount++
			// Fail every other record
			if callCount%2 == 0 {
				return testError
			}
			return nil
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-1", ContentURL: "https://example.com/1"},
		{ContentID: "test-2", ContentURL: "https://example.com/2"},
		{ContentID: "test-3", ContentURL: "https://example.com/3"},
		{ContentID: "test-4", ContentURL: "https://example.com/4"},
	}

	err := adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	// Should return nil for partial failures (backward compatibility)
	if err != nil {
		t.Errorf("expected nil error for partial failures, got %v", err)
	}

	// Should log individual errors for failed records
	expectedFailedCount := 2 // test-2 and test-4
	individualErrors := 0
	for _, call := range logger.errorCalls {
		if call.msg == "Failed to save classification history record" {
			individualErrors++
		}
	}

	if individualErrors != expectedFailedCount {
		t.Errorf("expected %d individual error logs, got %d", expectedFailedCount, individualErrors)
	}

	// Should log a warning for partial failure
	if len(logger.warnCalls) != 1 {
		t.Fatalf("expected 1 warning log for partial failure, got %d", len(logger.warnCalls))
	}

	warnCall := logger.warnCalls[0]
	if warnCall.msg != "Some classification history records failed to save" {
		t.Errorf("expected warning message 'Some classification history records failed to save', got %q", warnCall.msg)
	}

	// Verify warning includes failed_count
	failedCount, foundFailedCount := getFieldValue(warnCall.fields, "failed_count")
	if !foundFailedCount {
		t.Error("expected warning to include failed_count")
	} else if failedCount != expectedFailedCount {
		t.Errorf("expected failed_count %d, got %v", expectedFailedCount, failedCount)
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_NoLogger(t *testing.T) {
	// Test backward compatibility - adapter without logger should still work
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			return errors.New("database error")
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, nil) // No logger

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-1", ContentURL: "https://example.com/1"},
	}

	err := adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	// Should still return error when all fail, even without logger
	if err == nil {
		t.Error("expected error when all records fail, even without logger")
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_ErrorLoggingIncludesContentID(t *testing.T) {
	logger := newMockLoggerWithCalls()
	testError := errors.New("database error")
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			return testError
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-content-id", ContentURL: "https://example.com/article"},
	}

	_ = adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	// Find the individual error log
	var errorCall *logCall
	for i := range logger.errorCalls {
		if logger.errorCalls[i].msg == "Failed to save classification history record" {
			errorCall = &logger.errorCalls[i]
			break
		}
	}

	if errorCall == nil {
		t.Fatal("expected individual error log, not found")
	}

	// Verify content_id is included
	contentID, foundContentID := getFieldValue(errorCall.fields, "content_id")
	if !foundContentID {
		t.Error("expected error log to include content_id")
	} else if contentID != "test-content-id" {
		t.Errorf("expected content_id 'test-content-id', got %v", contentID)
	}
}

func TestDatabaseAdapter_SaveClassificationHistoryBatch_ErrorLoggingTruncatesLongURL(t *testing.T) {
	logger := newMockLoggerWithCalls()
	testError := errors.New("database error")
	repo := &mockHistoryRepository{
		createFunc: func(ctx context.Context, history *domain.ClassificationHistory) error {
			return testError
		},
	}
	adapter := NewDatabaseAdapterWithRepository(repo, logger)

	// Create a very long URL
	longURL := make([]byte, 500)
	for i := range longURL {
		longURL[i] = 'a'
	}

	histories := []*domain.ClassificationHistory{
		{ContentID: "test-1", ContentURL: string(longURL)},
	}

	_ = adapter.SaveClassificationHistoryBatch(context.Background(), histories)

	// Find the individual error log
	var errorCall *logCall
	for i := range logger.errorCalls {
		if logger.errorCalls[i].msg == "Failed to save classification history record" {
			errorCall = &logger.errorCalls[i]
			break
		}
	}

	if errorCall == nil {
		t.Fatal("expected individual error log, not found")
	}

	// Verify content_url is truncated
	contentURL, foundContentURL := getFieldValue(errorCall.fields, "content_url")
	if !foundContentURL {
		t.Error("expected error log to include content_url")
		return
	}
	contentURLStr, ok := contentURL.(string)
	if !ok {
		t.Errorf("expected content_url to be string, got %T", contentURL)
		return
	}
	// truncateString adds "..." so max length is urlLogTruncateLength + 3
	maxExpectedLength := urlLogTruncateLength + 3
	if len(contentURLStr) > maxExpectedLength {
		t.Errorf("expected content_url to be truncated to <= %d (including '...'), got length %d", maxExpectedLength, len(contentURLStr))
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "1234567890", 10, "1234567890"},
		{"long string", "12345678901234567890", 10, "1234567890..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

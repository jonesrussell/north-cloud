//nolint:testpackage // Testing internal processor requires same package access
package processor

import (
	"reflect"
	"testing"

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

func TestPoller_validateURL_ShortURL(t *testing.T) {
	logger := newMockLoggerWithCalls()
	poller := &Poller{logger: logger}

	url := "https://example.com/article"
	result := poller.validateURL(url)

	if result != url {
		t.Errorf("expected URL to remain unchanged, got %q", result)
	}

	if len(logger.warnCalls) != 0 {
		t.Errorf("expected no warning for short URL, got %d warnings", len(logger.warnCalls))
	}
}

func TestPoller_validateURL_ExactMaxLength(t *testing.T) {
	logger := newMockLoggerWithCalls()
	poller := &Poller{logger: logger}

	// Create URL exactly at maxURLLength
	url := make([]byte, maxURLLength)
	for i := range url {
		url[i] = 'a'
	}
	urlStr := string(url)

	result := poller.validateURL(urlStr)

	if result != urlStr {
		t.Errorf("expected URL to remain unchanged at exact max length, got length %d", len(result))
	}

	if len(logger.warnCalls) != 0 {
		t.Errorf("expected no warning for URL at exact max length, got %d warnings", len(logger.warnCalls))
	}
}

func TestPoller_validateURL_LongURL(t *testing.T) {
	logger := newMockLoggerWithCalls()
	poller := &Poller{logger: logger}

	// Create URL longer than maxURLLength
	longURL := make([]byte, maxURLLength+100)
	for i := range longURL {
		longURL[i] = 'a'
	}
	urlStr := string(longURL)

	result := poller.validateURL(urlStr)

	// Should be truncated to maxURLLength
	if len(result) != maxURLLength {
		t.Errorf("expected truncated URL length %d, got %d", maxURLLength, len(result))
	}

	// Should log a warning
	if len(logger.warnCalls) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(logger.warnCalls))
	}

	warnCall := logger.warnCalls[0]
	if warnCall.msg != "URL truncated for classification history" {
		t.Errorf("expected warning message 'URL truncated for classification history', got %q", warnCall.msg)
	}

	// Verify warning includes original_length
	originalLength, foundOriginalLength := getFieldValue(warnCall.fields, "original_length")
	if !foundOriginalLength {
		t.Error("expected warning to include original_length")
	} else if originalLength != len(urlStr) {
		t.Errorf("expected original_length %d, got %v", len(urlStr), originalLength)
	}
}

func TestPoller_validateURL_VeryLongURL(t *testing.T) {
	logger := newMockLoggerWithCalls()
	poller := &Poller{logger: logger}

	// Create a very long URL (5000 characters)
	veryLongURL := make([]byte, 5000)
	for i := range veryLongURL {
		veryLongURL[i] = 'b'
	}
	urlStr := string(veryLongURL)

	result := poller.validateURL(urlStr)

	// Should be truncated to maxURLLength
	if len(result) != maxURLLength {
		t.Errorf("expected truncated URL length %d, got %d", maxURLLength, len(result))
	}

	// Should log a warning
	if len(logger.warnCalls) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(logger.warnCalls))
	}

	// Verify URL preview is truncated to urlPreviewLength
	warnCall := logger.warnCalls[0]
	urlPreview, foundURLPreview := getFieldValue(warnCall.fields, "url_preview")
	if !foundURLPreview {
		t.Error("expected warning to include url_preview")
	} else {
		urlPreviewStr, ok := urlPreview.(string)
		if !ok {
			t.Errorf("expected url_preview to be string, got %T", urlPreview)
		} else if len(urlPreviewStr) > urlPreviewLength {
			t.Errorf("expected url_preview length <= %d, got %d", urlPreviewLength, len(urlPreviewStr))
		}
	}
}

func TestPoller_validateURL_EmptyURL(t *testing.T) {
	logger := newMockLoggerWithCalls()
	poller := &Poller{logger: logger}

	result := poller.validateURL("")

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}

	if len(logger.warnCalls) != 0 {
		t.Errorf("expected no warning for empty URL, got %d warnings", len(logger.warnCalls))
	}
}

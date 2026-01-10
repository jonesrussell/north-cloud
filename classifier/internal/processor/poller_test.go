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

// getFieldValue extracts the value from a zap.Field by key name.
// zapcore.Field structure: Key (string, index 0), Type (FieldType, index 1),
// Integer (int64, index 2), String (string, index 3), Interface (any, index 4)
func getFieldValue(fields []infralogger.Field, key string) (any, bool) {
	for _, field := range fields {
		fieldValue := reflect.ValueOf(field)
		if fieldValue.Kind() != reflect.Struct {
			continue
		}

		numFields := fieldValue.NumField()
		if numFields < 2 {
			continue
		}

		// Get Key field (index 0 should be the Key string)
		keyField := fieldValue.Field(0)
		if keyField.Kind() != reflect.String {
			continue
		}

		fieldKey := keyField.String()
		if fieldKey != key {
			continue
		}

		// Extract value based on field indices
		if val, found := extractFieldValue(fieldValue, numFields); found {
			return val, true
		}
	}
	return nil, false
}

// extractFieldValue extracts the value from a zap.Field based on field indices.
func extractFieldValue(fieldValue reflect.Value, numFields int) (any, bool) {
	// Check Integer field (index 2) for Int/Int64 fields first
	if numFields > 2 {
		if intVal, found := extractIntValue(fieldValue); found {
			return intVal, true
		}
	}

	// Check String field (index 3) for String fields
	if numFields > 3 {
		if strVal, found := extractStringValue(fieldValue, numFields); found {
			return strVal, true
		}
	}

	// Try Interface field (index 4) for other types
	if numFields > 4 {
		if ifaceVal, found := extractInterfaceValue(fieldValue); found {
			return ifaceVal, true
		}
	}

	// Fallback: Return Integer field even if zero (for Int fields with zero value)
	if numFields > 2 {
		intField := fieldValue.Field(2)
		if intField.Kind() == reflect.Int64 && intField.IsValid() {
			return intField.Int(), true
		}
	}

	return nil, false
}

// extractIntValue extracts integer value from field at index 2.
func extractIntValue(fieldValue reflect.Value) (any, bool) {
	intField := fieldValue.Field(2)
	if intField.Kind() == reflect.Int64 && intField.IsValid() {
		intVal := intField.Int()
		if intVal != 0 {
			return intVal, true
		}
	}
	return nil, false
}

// extractStringValue extracts string value from field at index 3.
func extractStringValue(fieldValue reflect.Value, numFields int) (any, bool) {
	strField := fieldValue.Field(3)
	if strField.Kind() != reflect.String || !strField.IsValid() {
		return nil, false
	}

	// For String fields, check if Integer field is zero (to distinguish from Int fields)
	if numFields > 2 {
		intField := fieldValue.Field(2)
		if intField.Kind() == reflect.Int64 && intField.Int() != 0 {
			// Integer field is non-zero, this is likely an Int field, not String
			return nil, false
		}
	}

	return strField.String(), true
}

// extractInterfaceValue extracts interface value from field at index 4.
func extractInterfaceValue(fieldValue reflect.Value) (any, bool) {
	ifaceField := fieldValue.Field(4)
	if ifaceField.Kind() == reflect.Interface && ifaceField.IsValid() && !ifaceField.IsNil() {
		ifaceVal := ifaceField.Interface()
		if ifaceVal != nil {
			return ifaceVal, true
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
	} else {
		// Handle type conversion (zap stores int as int64)
		var originalLengthInt int
		switch v := originalLength.(type) {
		case int:
			originalLengthInt = v
		case int64:
			originalLengthInt = int(v)
		default:
			t.Errorf("expected original_length to be int or int64, got %T", originalLength)
			return
		}
		if originalLengthInt != len(urlStr) {
			t.Errorf("expected original_length %d, got %d", len(urlStr), originalLengthInt)
		}
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

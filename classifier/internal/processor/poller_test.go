//nolint:testpackage // Testing internal processor requires same package access
package processor

import (
	"testing"
)

// mockLoggerWithCalls tracks log calls for testing
type mockLoggerWithCalls struct {
	debugCalls []logCall
	infoCalls  []logCall
	warnCalls  []logCall
	errorCalls []logCall
}

type logCall struct {
	msg           string
	keysAndValues []interface{}
}

func newMockLoggerWithCalls() *mockLoggerWithCalls {
	return &mockLoggerWithCalls{
		debugCalls: make([]logCall, 0),
		infoCalls:  make([]logCall, 0),
		warnCalls:  make([]logCall, 0),
		errorCalls: make([]logCall, 0),
	}
}

func (m *mockLoggerWithCalls) Debug(msg string, keysAndValues ...interface{}) {
	m.debugCalls = append(m.debugCalls, logCall{msg: msg, keysAndValues: keysAndValues})
}

func (m *mockLoggerWithCalls) Info(msg string, keysAndValues ...interface{}) {
	m.infoCalls = append(m.infoCalls, logCall{msg: msg, keysAndValues: keysAndValues})
}

func (m *mockLoggerWithCalls) Warn(msg string, keysAndValues ...interface{}) {
	m.warnCalls = append(m.warnCalls, logCall{msg: msg, keysAndValues: keysAndValues})
}

func (m *mockLoggerWithCalls) Error(msg string, keysAndValues ...interface{}) {
	m.errorCalls = append(m.errorCalls, logCall{msg: msg, keysAndValues: keysAndValues})
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
	foundOriginalLength := false
	for i := 0; i < len(warnCall.keysAndValues); i += 2 {
		if i+1 < len(warnCall.keysAndValues) {
			key := warnCall.keysAndValues[i]
			if key == "original_length" {
				foundOriginalLength = true
				value := warnCall.keysAndValues[i+1]
				if value != len(urlStr) {
					t.Errorf("expected original_length %d, got %v", len(urlStr), value)
				}
				break
			}
		}
	}

	if !foundOriginalLength {
		t.Error("expected warning to include original_length")
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
	foundURLPreview := false
	for i := 0; i < len(warnCall.keysAndValues); i += 2 {
		if i+1 < len(warnCall.keysAndValues) {
			key := warnCall.keysAndValues[i]
			if key == "url_preview" {
				foundURLPreview = true
				value := warnCall.keysAndValues[i+1].(string)
				if len(value) > urlPreviewLength {
					t.Errorf("expected url_preview length <= %d, got %d", urlPreviewLength, len(value))
				}
				break
			}
		}
	}

	if !foundURLPreview {
		t.Error("expected warning to include url_preview")
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

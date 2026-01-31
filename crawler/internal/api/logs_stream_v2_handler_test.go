package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

func TestLogsStreamV2Handler_Stream(t *testing.T) {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	// Setup
	jobID := "test-v2-stream-" + time.Now().Format("20060102150405")
	streamKey := "logs:" + jobID
	defer client.Del(context.Background(), streamKey)

	writer := logs.NewRedisStreamWriter(client, "logs", 3600)
	logger := infralogger.NewNop()

	handler := api.NewLogsStreamV2Handler(writer, logger)

	// Write test entries before connecting
	for i := range 3 {
		entry := logs.LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "Test message " + string(rune('0'+i)),
			JobID:     jobID,
			ExecID:    "exec-1",
		}
		if err := writer.WriteEntry(context.Background(), entry); err != nil {
			t.Fatalf("WriteEntry failed: %v", err)
		}
	}

	// Create request
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: jobID}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID+"/logs/stream/v2", http.NoBody)
	c.Request = req

	// Use a context with timeout to avoid blocking forever
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)

	// Call handler (will timeout after 100ms)
	handler.Stream(c)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check SSE headers
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cacheControl)
	}

	// Verify we got some SSE data (connected event + replay)
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	// Should contain event types
	if !strings.Contains(body, "event:") {
		t.Error("expected SSE event in response body")
	}
}

func TestLogsStreamV2Handler_MissingJobID(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNop()
	handler := api.NewLogsStreamV2Handler(nil, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{} // No job ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs//logs/stream/v2", http.NoBody)
	c.Request = req

	handler.Stream(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogsStreamV2Handler_NoRedisWriter(t *testing.T) {
	t.Helper()

	logger := infralogger.NewNop()
	handler := api.NewLogsStreamV2Handler(nil, logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "some-job"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/some-job/logs/stream/v2", http.NoBody)
	c.Request = req

	handler.Stream(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

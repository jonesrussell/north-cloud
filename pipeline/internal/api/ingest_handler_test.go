package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/api"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

type mockPipelineService struct {
	ingestFunc      func(req *domain.IngestRequest) error
	ingestBatchFunc func(req *domain.BatchIngestRequest) (int, error)
}

func (m *mockPipelineService) Ingest(_ context.Context, req *domain.IngestRequest) error {
	if m.ingestFunc != nil {
		return m.ingestFunc(req)
	}
	return nil
}

func (m *mockPipelineService) IngestBatch(_ context.Context, req *domain.BatchIngestRequest) (int, error) {
	if m.ingestBatchFunc != nil {
		return m.ingestBatchFunc(req)
	}
	return len(req.Events), nil
}

func (m *mockPipelineService) GetFunnel(_ context.Context, _, _ time.Time) (*domain.FunnelResponse, error) {
	return &domain.FunnelResponse{}, nil
}

func setupTestRouter(t *testing.T, handler *api.IngestHandler) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.POST("/events", handler.IngestEvent)
	v1.POST("/events/batch", handler.IngestBatch)

	return router
}

func TestIngestHandler_IngestEvent_Success(t *testing.T) {
	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	body := map[string]any{
		"article_url":  "https://example.com/article",
		"source_name":  "example_com",
		"stage":        "crawled",
		"occurred_at":  time.Now().UTC().Format(time.RFC3339),
		"service_name": "crawler",
	}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestIngestHandler_IngestEvent_BadRequest(t *testing.T) {
	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	// Missing required fields
	body := map[string]any{"article_url": "https://example.com"}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIngestHandler_IngestEvent_ServiceError(t *testing.T) {
	svc := &mockPipelineService{
		ingestFunc: func(_ *domain.IngestRequest) error {
			return errors.New("service error")
		},
	}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	body := map[string]any{
		"article_url":  "https://example.com/article",
		"source_name":  "example_com",
		"stage":        "crawled",
		"occurred_at":  time.Now().UTC().Format(time.RFC3339),
		"service_name": "crawler",
	}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIngestHandler_IngestBatch_Success(t *testing.T) {
	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	body := map[string]any{
		"events": []map[string]any{
			{
				"article_url":  "https://example.com/article-1",
				"source_name":  "example_com",
				"stage":        "crawled",
				"occurred_at":  time.Now().UTC().Format(time.RFC3339),
				"service_name": "crawler",
			},
			{
				"article_url":  "https://example.com/article-2",
				"source_name":  "example_com",
				"stage":        "classified",
				"occurred_at":  time.Now().UTC().Format(time.RFC3339),
				"service_name": "classifier",
			},
		},
	}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events/batch", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]any
	if decodeErr := json.NewDecoder(w.Body).Decode(&resp); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}

	expectedEvents := 2
	if ingested, ok := resp["ingested"].(float64); !ok || int(ingested) != expectedEvents {
		t.Errorf("ingested = %v, want %d", resp["ingested"], expectedEvents)
	}
}

func TestIngestHandler_IngestBatch_ServiceError(t *testing.T) {
	svc := &mockPipelineService{
		ingestBatchFunc: func(_ *domain.BatchIngestRequest) (int, error) {
			return 1, errors.New("batch error at event 1")
		},
	}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	body := map[string]any{
		"events": []map[string]any{
			{
				"article_url":  "https://example.com/article-1",
				"source_name":  "example_com",
				"stage":        "crawled",
				"occurred_at":  time.Now().UTC().Format(time.RFC3339),
				"service_name": "crawler",
			},
		},
	}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events/batch", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIngestHandler_IngestBatch_BadRequest(t *testing.T) {
	svc := &mockPipelineService{}
	handler := api.NewIngestHandler(svc)
	router := setupTestRouter(t, handler)

	// Empty events array violates min=1 binding
	body := map[string]any{"events": []map[string]any{}}

	bodyJSON, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		t.Fatalf("failed to marshal body: %v", marshalErr)
	}

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/events/batch", bytes.NewBuffer(bodyJSON))
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

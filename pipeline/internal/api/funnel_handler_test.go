package api_test

import (
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

func setupFunnelRouter(t *testing.T, handler *api.FunnelHandler) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/funnel", handler.GetFunnel)

	return router
}

type mockFunnelService struct {
	getFunnelFunc func(from, to time.Time) (*domain.FunnelResponse, error)
}

func (m *mockFunnelService) GetFunnel(_ context.Context, from, to time.Time) (*domain.FunnelResponse, error) {
	if m.getFunnelFunc != nil {
		return m.getFunnelFunc(from, to)
	}

	return &domain.FunnelResponse{
		Stages:      []domain.FunnelStage{},
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func TestFunnelHandler_GetFunnel_DefaultPeriod(t *testing.T) {
	svc := &mockFunnelService{
		getFunnelFunc: func(_, _ time.Time) (*domain.FunnelResponse, error) {
			return &domain.FunnelResponse{
				Period:   "today",
				Timezone: "UTC",
				Stages: []domain.FunnelStage{
					{Name: "crawled", Count: 100, UniqueArticles: 90},
				},
				GeneratedAt: time.Now().UTC(),
			}, nil
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(t, handler)

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/funnel", http.NoBody)
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp domain.FunnelResponse
	if decodeErr := json.NewDecoder(w.Body).Decode(&resp); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}

	if resp.Period != "today" {
		t.Errorf("period = %q, want %q", resp.Period, "today")
	}

	if resp.Timezone != "UTC" {
		t.Errorf("timezone = %q, want %q", resp.Timezone, "UTC")
	}

	expectedStages := 1
	if len(resp.Stages) != expectedStages {
		t.Errorf("stages count = %d, want %d", len(resp.Stages), expectedStages)
	}
}

func TestFunnelHandler_GetFunnel_24hPeriod(t *testing.T) {
	var capturedFrom, capturedTo time.Time

	svc := &mockFunnelService{
		getFunnelFunc: func(from, to time.Time) (*domain.FunnelResponse, error) {
			capturedFrom = from
			capturedTo = to

			return &domain.FunnelResponse{
				Stages:      []domain.FunnelStage{},
				GeneratedAt: time.Now().UTC(),
			}, nil
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(t, handler)

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/funnel?period=24h", http.NoBody)
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify the time range is approximately 24 hours
	duration := capturedTo.Sub(capturedFrom)
	expectedHours := 24

	if int(duration.Hours()) != expectedHours {
		t.Errorf("duration hours = %v, want %d", duration.Hours(), expectedHours)
	}
}

func TestFunnelHandler_GetFunnel_7dPeriod(t *testing.T) {
	var capturedFrom time.Time

	svc := &mockFunnelService{
		getFunnelFunc: func(from, _ time.Time) (*domain.FunnelResponse, error) {
			capturedFrom = from

			return &domain.FunnelResponse{
				Stages:      []domain.FunnelStage{},
				GeneratedAt: time.Now().UTC(),
			}, nil
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(t, handler)

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/funnel?period=7d", http.NoBody)
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify the from time is approximately 7 days ago
	expectedDays := 7
	daysDiff := int(time.Since(capturedFrom).Hours() / hoursPerDay)

	if daysDiff != expectedDays {
		t.Errorf("days diff = %d, want %d", daysDiff, expectedDays)
	}
}

func TestFunnelHandler_GetFunnel_30dPeriod(t *testing.T) {
	var capturedFrom time.Time

	svc := &mockFunnelService{
		getFunnelFunc: func(from, _ time.Time) (*domain.FunnelResponse, error) {
			capturedFrom = from

			return &domain.FunnelResponse{
				Stages:      []domain.FunnelStage{},
				GeneratedAt: time.Now().UTC(),
			}, nil
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(t, handler)

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/funnel?period=30d", http.NoBody)
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify the from time is approximately 30 days ago
	expectedDays := 30
	daysDiff := int(time.Since(capturedFrom).Hours() / hoursPerDay)

	if daysDiff != expectedDays {
		t.Errorf("days diff = %d, want %d", daysDiff, expectedDays)
	}
}

func TestFunnelHandler_GetFunnel_ServiceError(t *testing.T) {
	svc := &mockFunnelService{
		getFunnelFunc: func(_, _ time.Time) (*domain.FunnelResponse, error) {
			return nil, errors.New("database connection failed")
		},
	}

	handler := api.NewFunnelHandler(svc)
	router := setupFunnelRouter(t, handler)

	w := httptest.NewRecorder()

	req, reqErr := http.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/funnel", http.NoBody)
	if reqErr != nil {
		t.Fatalf("failed to create request: %v", reqErr)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// hoursPerDay is the number of hours in a day (used for day calculations in tests).
const hoursPerDay = 24

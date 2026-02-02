package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testServer creates a test server with the proxy and admin handlers.
func testServer(t *testing.T) (*httptest.Server, *Proxy) {
	t.Helper()

	cfg := &Config{
		Port:        8055,
		Mode:        ModeReplay,
		FixturesDir: t.TempDir(),
		CacheDir:    t.TempDir(),
	}

	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	mux := http.NewServeMux()
	mux.Handle("/admin/", admin)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.Handle("/", proxy)

	return httptest.NewServer(mux), proxy
}

// TestIntegration_HealthCheck tests the health endpoint.
func TestIntegration_HealthCheck(t *testing.T) {
	server, _ := testServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("expected body 'OK', got '%s'", body)
	}
}

// TestIntegration_AdminStatus tests the status endpoint.
func TestIntegration_AdminStatus(t *testing.T) {
	server, _ := testServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/status")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var status StatusResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&status); decodeErr != nil {
		t.Fatalf("failed to decode status: %v", decodeErr)
	}

	if status.Mode != "replay" {
		t.Errorf("expected mode 'replay', got '%s'", status.Mode)
	}
}

// TestIntegration_ModeSwitching tests mode switching via admin API.
func TestIntegration_ModeSwitching(t *testing.T) {
	server, _ := testServer(t)
	defer server.Close()

	// Switch to record mode
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/admin/mode/record", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mode switch failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify mode changed
	statusResp, _ := http.Get(server.URL + "/admin/status")
	defer func() { _ = statusResp.Body.Close() }()

	var status StatusResponse
	if decodeErr := json.NewDecoder(statusResp.Body).Decode(&status); decodeErr != nil {
		t.Fatalf("failed to decode status: %v", decodeErr)
	}

	if status.Mode != "record" {
		t.Errorf("expected mode 'record', got '%s'", status.Mode)
	}
}

// TestIntegration_InvalidMode tests invalid mode returns error.
func TestIntegration_InvalidMode(t *testing.T) {
	server, _ := testServer(t)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/admin/mode/invalid", http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// TestIntegration_CacheList tests cache listing endpoint.
func TestIntegration_CacheList(t *testing.T) {
	server, _ := testServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin/cache")
	if err != nil {
		t.Fatalf("cache list failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var domains []string
	if decodeErr := json.NewDecoder(resp.Body).Decode(&domains); decodeErr != nil {
		t.Fatalf("failed to decode domains: %v", decodeErr)
	}

	if len(domains) != 0 {
		t.Errorf("expected empty domains, got %v", domains)
	}
}

// TestIntegration_ReplayModeCacheMiss tests replay mode returns 502 on cache miss.
func TestIntegration_ReplayModeCacheMiss(t *testing.T) {
	_, proxy := testServer(t)
	proxy.SetMode(ModeReplay)

	req := httptest.NewRequest(http.MethodGet, "http://uncached-domain.com/page", http.NoBody)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

// TestIntegration_LiveModeAttempts tests live mode attempts real request.
func TestIntegration_LiveModeAttempts(t *testing.T) {
	_, proxy := testServer(t)
	proxy.SetMode(ModeLive)

	// Request to a non-existent domain should fail
	req := httptest.NewRequest(http.MethodGet, "http://non-existent-domain-xyz123.invalid/page", http.NoBody)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should get an error (502) because domain doesn't exist
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for non-existent domain, got %d", w.Code)
	}
}

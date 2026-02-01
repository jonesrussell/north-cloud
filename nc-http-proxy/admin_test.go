package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminStatus(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cfg := &Config{
		Mode:        ModeReplay,
		FixturesDir: fixturesDir,
		CacheDir:    t.TempDir(),
	}
	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodGet, "/admin/status", http.NoBody)
	w := httptest.NewRecorder()

	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var status StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if status.Mode != "replay" {
		t.Errorf("expected mode replay, got %s", status.Mode)
	}
}

func TestAdminModeSwitch(t *testing.T) {
	t.Helper()
	cfg := &Config{
		Mode:        ModeReplay,
		FixturesDir: t.TempDir(),
		CacheDir:    t.TempDir(),
	}
	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	// Switch to record mode
	req := httptest.NewRequest(http.MethodPost, "/admin/mode/record", http.NoBody)
	w := httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for mode switch, got %d", w.Code)
	}

	if proxy.Mode() != ModeRecord {
		t.Errorf("expected mode record, got %s", proxy.Mode())
	}

	// Switch to live mode
	req = httptest.NewRequest(http.MethodPost, "/admin/mode/live", http.NoBody)
	w = httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if proxy.Mode() != ModeLive {
		t.Errorf("expected mode live, got %s", proxy.Mode())
	}

	// Switch back to replay
	req = httptest.NewRequest(http.MethodPost, "/admin/mode/replay", http.NoBody)
	w = httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if proxy.Mode() != ModeReplay {
		t.Errorf("expected mode replay, got %s", proxy.Mode())
	}
}

func TestAdminInvalidMode(t *testing.T) {
	t.Helper()
	cfg := &Config{Mode: ModeReplay, FixturesDir: t.TempDir(), CacheDir: t.TempDir()}
	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodPost, "/admin/mode/invalid", http.NoBody)
	w := httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid mode, got %d", w.Code)
	}
}

func TestAdminListCache(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cfg := &Config{
		Mode:        ModeReplay,
		FixturesDir: fixturesDir,
		CacheDir:    t.TempDir(),
	}
	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodGet, "/admin/cache", http.NoBody)
	w := httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var domains []string
	if err := json.Unmarshal(w.Body.Bytes(), &domains); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(domains) == 0 {
		t.Error("expected at least one domain")
	}
}

func TestAdminNotFound(t *testing.T) {
	t.Helper()
	cfg := &Config{Mode: ModeReplay, FixturesDir: t.TempDir(), CacheDir: t.TempDir()}
	proxy := NewProxy(cfg)
	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodGet, "/admin/unknown", http.NoBody)
	w := httptest.NewRecorder()
	admin.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown endpoint, got %d", w.Code)
	}
}

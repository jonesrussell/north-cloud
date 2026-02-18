package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// errPathTraversal is returned when a path traversal attempt is detected.
var errPathTraversal = errors.New("path traversal attempt detected")

// StatusResponse is the response for GET /admin/status.
type StatusResponse struct {
	Mode          string   `json:"mode"`
	FixturesCount int      `json:"fixtures_count"`
	CacheCount    int      `json:"cache_count"`
	Domains       []string `json:"domains"`
}

// AdminHandler handles admin API requests.
type AdminHandler struct {
	proxy *Proxy
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(proxy *Proxy) *AdminHandler {
	return &AdminHandler{proxy: proxy}
}

// ServeHTTP routes admin requests.
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/admin/status" && r.Method == http.MethodGet:
		h.handleStatus(w)
	case strings.HasPrefix(path, "/admin/mode/") && r.Method == http.MethodPost:
		h.handleModeSwitch(w, r)
	case path == "/admin/cache" && r.Method == http.MethodGet:
		h.handleListCache(w)
	case strings.HasPrefix(path, "/admin/cache/") && r.Method == http.MethodGet:
		h.handleListDomainCache(w, r)
	case path == "/admin/cache" && r.Method == http.MethodDelete:
		h.handleClearCache(w)
	case strings.HasPrefix(path, "/admin/cache/") && r.Method == http.MethodDelete:
		h.handleClearDomainCache(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *AdminHandler) handleStatus(w http.ResponseWriter) {
	stats := h.proxy.Cache().Stats()

	response := StatusResponse{
		Mode:          string(h.proxy.Mode()),
		FixturesCount: stats.FixturesCount,
		CacheCount:    stats.CacheCount,
		Domains:       stats.Domains,
	}

	h.writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandler) handleModeSwitch(w http.ResponseWriter, r *http.Request) {
	modeStr := strings.TrimPrefix(r.URL.Path, "/admin/mode/")
	mode := Mode(modeStr)

	if !mode.IsValid() {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_mode",
			"message": fmt.Sprintf("Invalid mode: %s. Valid modes: replay, record, live", modeStr),
		})
		return
	}

	h.proxy.SetMode(mode)

	h.writeJSON(w, http.StatusOK, map[string]string{
		"mode":    string(mode),
		"message": "Mode switched successfully",
	})
}

func (h *AdminHandler) handleListCache(w http.ResponseWriter) {
	stats := h.proxy.Cache().Stats()
	h.writeJSON(w, http.StatusOK, stats.Domains)
}

func (h *AdminHandler) handleListDomainCache(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimPrefix(r.URL.Path, "/admin/cache/")
	entries := h.listDomainEntries(domain)
	h.writeJSON(w, http.StatusOK, entries)
}

func (h *AdminHandler) handleClearCache(w http.ResponseWriter) {
	cacheDir := h.proxy.Cache().CacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		h.writeJSON(w, http.StatusOK, map[string]string{"message": "Cache cleared"})
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			_ = os.RemoveAll(filepath.Join(cacheDir, entry.Name())) //nolint:gosec // G703: cacheDir is config-controlled, entry from ReadDir
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "Cache cleared"})
}

func (h *AdminHandler) handleClearDomainCache(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimPrefix(r.URL.Path, "/admin/cache/")

	// Validate path to prevent traversal attacks
	domainDir, err := safePath(h.proxy.Cache().CacheDir(), domain)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain"})
		return
	}

	_ = os.RemoveAll(domainDir) //nolint:gosec // G703: domainDir validated by safePath

	h.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Cache cleared for " + domain,
	})
}

func (h *AdminHandler) listDomainEntries(domain string) []string {
	entries := make([]string, 0)

	// Check fixtures - validate path to prevent traversal
	if fixturesDir, err := safePath(h.proxy.Cache().FixturesDir(), domain); err == nil {
		h.appendEntriesFromDir(fixturesDir, &entries)
	}

	// Check cache - validate path to prevent traversal
	if cacheDir, err := safePath(h.proxy.Cache().CacheDir(), domain); err == nil {
		h.appendEntriesFromDir(cacheDir, &entries)
	}

	return entries
}

func (h *AdminHandler) appendEntriesFromDir(dir string, entries *[]string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			cacheKey := strings.TrimSuffix(file.Name(), ".json")
			*entries = append(*entries, cacheKey)
		}
	}
}

func (h *AdminHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(data); encodeErr != nil {
		fmt.Printf("warning: failed to encode JSON response: %v\n", encodeErr)
	}
}

// safePath joins baseDir with untrusted input and returns the result only if
// it remains within baseDir. This prevents path traversal attacks.
func safePath(baseDir, untrusted string) (string, error) {
	// Clean the untrusted input to remove any path traversal attempts
	cleaned := filepath.Clean(untrusted)

	// Reject obvious traversal attempts
	if strings.Contains(cleaned, "..") {
		return "", errPathTraversal
	}

	// Join with base and verify result stays within base
	result := filepath.Join(baseDir, cleaned)

	// Ensure the result starts with the base directory
	// Use Clean on baseDir too for consistent comparison
	cleanBase := filepath.Clean(baseDir)
	if !strings.HasPrefix(result, cleanBase+string(filepath.Separator)) && result != cleanBase {
		return "", errPathTraversal
	}

	return result, nil
}

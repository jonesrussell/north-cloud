# MCP Server - Proxy Control Tools Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 9 MCP tools for controlling the nc-http-proxy service, enabling AI agents to manage proxy modes, domain overrides, cache, and audit logs.

**Architecture:** MCP server calls nc-http-proxy admin HTTP endpoints via a new ProxyClient. The proxy is enhanced with per-domain mode overrides, hybrid fallback mode, and an audit log.

**Tech Stack:** Go 1.24+, HTTP REST, JSON-RPC 2.0 (MCP protocol)

---

## Prerequisites

Before starting, ensure:
- `task docker:dev:up` is running
- `task proxy:up` starts the proxy on port 8055
- You can run `curl http://localhost:8055/admin/status` successfully

---

## Task 1: Add Per-Domain Override Support to Proxy Config

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/config.go`
- Test: `/home/fsd42/dev/north-cloud/nc-http-proxy/config_test.go`

**Step 1.1: Write failing test for domain overrides in config**

```go
// Add to config_test.go
func TestConfig_DomainOverrides(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
	}

	// Test adding override
	cfg.DomainOverrides["example.com"] = ModeRecord

	if cfg.DomainOverrides["example.com"] != ModeRecord {
		t.Errorf("expected ModeRecord, got %s", cfg.DomainOverrides["example.com"])
	}
}
```

**Step 1.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestConfig_DomainOverrides -v`
Expected: FAIL - `DomainOverrides` field doesn't exist

**Step 1.3: Add DomainOverrides field to Config struct**

In `config.go`, modify the `Config` struct:

```go
type Config struct {
	Port           int
	Mode           Mode
	FixturesDir    string
	CacheDir       string
	CertsDir       string
	LiveTimeout    time.Duration
	DomainOverrides map[string]Mode // NEW: per-domain mode overrides
	HybridFallback  bool            // NEW: enable replay->record fallback
}
```

**Step 1.4: Update LoadConfig to initialize map**

In `config.go`, update `LoadConfig()`:

```go
func LoadConfig() *Config {
	// ... existing code ...

	return &Config{
		Port:            port,
		Mode:            mode,
		FixturesDir:     fixturesDir,
		CacheDir:        cacheDir,
		CertsDir:        certsDir,
		LiveTimeout:     liveTimeout,
		DomainOverrides: make(map[string]Mode), // NEW
		HybridFallback:  false,                  // NEW
	}
}
```

**Step 1.5: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestConfig_DomainOverrides -v`
Expected: PASS

**Step 1.6: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run`
Expected: No errors

**Step 1.7: Commit**

```bash
git add nc-http-proxy/config.go nc-http-proxy/config_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): add domain overrides and hybrid fallback config fields

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add Domain Override Methods to Proxy

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy.go`
- Test: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy_test.go`

**Step 2.1: Write failing test for GetModeForDomain**

```go
// Add to proxy_test.go
func TestProxy_GetModeForDomain(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	// Global mode when no override
	if mode := proxy.GetModeForDomain("example.com"); mode != ModeReplay {
		t.Errorf("expected ModeReplay, got %s", mode)
	}

	// Set domain override
	proxy.SetDomainMode("example.com", ModeRecord)

	if mode := proxy.GetModeForDomain("example.com"); mode != ModeRecord {
		t.Errorf("expected ModeRecord after override, got %s", mode)
	}

	// Other domains still use global
	if mode := proxy.GetModeForDomain("other.com"); mode != ModeReplay {
		t.Errorf("expected ModeReplay for other domain, got %s", mode)
	}
}
```

**Step 2.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_GetModeForDomain -v`
Expected: FAIL - methods don't exist

**Step 2.3: Add domain override methods to Proxy**

In `proxy.go`, add these methods:

```go
// GetModeForDomain returns the effective mode for a domain.
// Domain overrides take precedence over global mode.
func (p *Proxy) GetModeForDomain(domain string) Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if override, ok := p.cfg.DomainOverrides[domain]; ok {
		return override
	}
	return p.mode
}

// SetDomainMode sets a per-domain mode override.
func (p *Proxy) SetDomainMode(domain string, mode Mode) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cfg.DomainOverrides == nil {
		p.cfg.DomainOverrides = make(map[string]Mode)
	}
	p.cfg.DomainOverrides[domain] = mode
}

// ClearDomainMode removes a per-domain mode override.
func (p *Proxy) ClearDomainMode(domain string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.cfg.DomainOverrides, domain)
}

// GetDomainOverrides returns a copy of all domain overrides.
func (p *Proxy) GetDomainOverrides() map[string]Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]Mode, len(p.cfg.DomainOverrides))
	for k, v := range p.cfg.DomainOverrides {
		result[k] = v
	}
	return result
}
```

**Step 2.4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_GetModeForDomain -v`
Expected: PASS

**Step 2.5: Write test for ClearDomainMode**

```go
func TestProxy_ClearDomainMode(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	proxy.SetDomainMode("example.com", ModeRecord)
	proxy.ClearDomainMode("example.com")

	if mode := proxy.GetModeForDomain("example.com"); mode != ModeReplay {
		t.Errorf("expected ModeReplay after clear, got %s", mode)
	}
}
```

**Step 2.6: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_ClearDomainMode -v`
Expected: PASS

**Step 2.7: Run linter and all tests**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run && go test ./...`
Expected: All pass

**Step 2.8: Commit**

```bash
git add nc-http-proxy/proxy.go nc-http-proxy/proxy_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): add per-domain mode override methods

Adds GetModeForDomain, SetDomainMode, ClearDomainMode, and
GetDomainOverrides methods with thread-safe access.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Add Hybrid Fallback Mode to Proxy

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy.go`
- Test: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy_test.go`

**Step 3.1: Write failing test for hybrid fallback**

```go
func TestProxy_HybridFallback(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:           ModeReplay,
		HybridFallback: false,
		FixturesDir:    t.TempDir(),
		CacheDir:       t.TempDir(),
		CertsDir:       t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	if proxy.HybridFallback() {
		t.Error("expected hybrid fallback to be false initially")
	}

	proxy.SetHybridFallback(true)

	if !proxy.HybridFallback() {
		t.Error("expected hybrid fallback to be true after setting")
	}
}
```

**Step 3.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_HybridFallback -v`
Expected: FAIL - methods don't exist

**Step 3.3: Add hybrid fallback methods**

In `proxy.go`, add:

```go
// HybridFallback returns whether hybrid mode is enabled.
// In hybrid mode, replay misses fall through to record behavior.
func (p *Proxy) HybridFallback() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg.HybridFallback
}

// SetHybridFallback enables or disables hybrid fallback mode.
func (p *Proxy) SetHybridFallback(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg.HybridFallback = enabled
}
```

**Step 3.4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_HybridFallback -v`
Expected: PASS

**Step 3.5: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run`
Expected: No errors

**Step 3.6: Commit**

```bash
git add nc-http-proxy/proxy.go nc-http-proxy/proxy_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): add hybrid fallback mode support

Hybrid mode allows replay misses to fall through to record behavior.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Add Audit Log to Proxy

**Files:**
- Create: `/home/fsd42/dev/north-cloud/nc-http-proxy/audit.go`
- Create: `/home/fsd42/dev/north-cloud/nc-http-proxy/audit_test.go`

**Step 4.1: Write failing test for audit log**

Create `audit_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestAuditLog_RecordModeChange(t *testing.T) {
	t.Helper()

	audit := NewAuditLog(maxAuditEntries)

	audit.RecordModeChange("admin", ModeReplay, ModeRecord, "")

	entries := audit.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.User != "admin" {
		t.Errorf("expected user 'admin', got %s", entry.User)
	}
	if entry.OldMode != string(ModeReplay) {
		t.Errorf("expected old mode 'replay', got %s", entry.OldMode)
	}
	if entry.NewMode != string(ModeRecord) {
		t.Errorf("expected new mode 'record', got %s", entry.NewMode)
	}
	if entry.Domain != "" {
		t.Errorf("expected empty domain for global change, got %s", entry.Domain)
	}
}

func TestAuditLog_DomainModeChange(t *testing.T) {
	t.Helper()

	audit := NewAuditLog(maxAuditEntries)

	audit.RecordModeChange("admin", ModeReplay, ModeRecord, "example.com")

	entries := audit.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Domain != "example.com" {
		t.Errorf("expected domain 'example.com', got %s", entries[0].Domain)
	}
}

func TestAuditLog_MaxEntries(t *testing.T) {
	t.Helper()

	const maxEntries = 5
	audit := NewAuditLog(maxEntries)

	// Add more than max entries
	for i := 0; i < maxEntries+3; i++ {
		audit.RecordModeChange("admin", ModeReplay, ModeRecord, "")
		time.Sleep(time.Millisecond) // Ensure distinct timestamps
	}

	entries := audit.GetEntries()
	if len(entries) != maxEntries {
		t.Errorf("expected %d entries (max), got %d", maxEntries, len(entries))
	}
}
```

**Step 4.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestAuditLog -v`
Expected: FAIL - types don't exist

**Step 4.3: Implement audit log**

Create `audit.go`:

```go
package main

import (
	"sync"
	"time"
)

const maxAuditEntries = 100

// ModeChangeAudit represents a single mode change event.
type ModeChangeAudit struct {
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	OldMode   string    `json:"old_mode"`
	NewMode   string    `json:"new_mode"`
	Domain    string    `json:"domain,omitempty"` // empty = global change
}

// AuditLog maintains a ring buffer of mode change events.
type AuditLog struct {
	mu         sync.RWMutex
	entries    []ModeChangeAudit
	maxEntries int
}

// NewAuditLog creates a new audit log with the specified capacity.
func NewAuditLog(maxEntries int) *AuditLog {
	return &AuditLog{
		entries:    make([]ModeChangeAudit, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

// RecordModeChange adds a mode change event to the audit log.
func (a *AuditLog) RecordModeChange(user string, oldMode, newMode Mode, domain string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := ModeChangeAudit{
		Timestamp: time.Now(),
		User:      user,
		OldMode:   string(oldMode),
		NewMode:   string(newMode),
		Domain:    domain,
	}

	a.entries = append(a.entries, entry)

	// Trim to max entries (keep most recent)
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[len(a.entries)-a.maxEntries:]
	}
}

// GetEntries returns a copy of all audit entries.
func (a *AuditLog) GetEntries() []ModeChangeAudit {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]ModeChangeAudit, len(a.entries))
	copy(result, a.entries)
	return result
}

// Clear removes all audit entries.
func (a *AuditLog) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = a.entries[:0]
}
```

**Step 4.4: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestAuditLog -v`
Expected: PASS

**Step 4.5: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run`
Expected: No errors

**Step 4.6: Commit**

```bash
git add nc-http-proxy/audit.go nc-http-proxy/audit_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): add audit log for mode changes

Ring buffer stores up to 100 mode change events with user,
timestamp, old/new modes, and optional domain.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Integrate Audit Log with Proxy

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy.go`
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/main.go`
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/proxy_test.go`

**Step 5.1: Write failing test for proxy audit integration**

Add to `proxy_test.go`:

```go
func TestProxy_AuditIntegration(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	// Change global mode
	proxy.SetModeWithAudit(ModeRecord, "test-user")

	entries := proxy.GetAuditLog()
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}

	if entries[0].NewMode != string(ModeRecord) {
		t.Errorf("expected new mode 'record', got %s", entries[0].NewMode)
	}
}
```

**Step 5.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_AuditIntegration -v`
Expected: FAIL - methods don't exist

**Step 5.3: Add audit field to Proxy struct**

In `proxy.go`, update the Proxy struct:

```go
type Proxy struct {
	cfg      *Config
	cache    *Cache
	mu       sync.RWMutex
	mode     Mode
	client   *http.Client
	certMgr  *CertManager
	audit    *AuditLog // NEW
}
```

**Step 5.4: Initialize audit in NewProxy**

In `proxy.go`, update `NewProxy()`:

```go
func NewProxy(cfg *Config) (*Proxy, error) {
	// ... existing code ...

	return &Proxy{
		cfg:     cfg,
		cache:   cache,
		mode:    cfg.Mode,
		client:  client,
		certMgr: certMgr,
		audit:   NewAuditLog(maxAuditEntries), // NEW
	}, nil
}
```

**Step 5.5: Add audit-aware mode change methods**

In `proxy.go`, add:

```go
// SetModeWithAudit changes the global mode and records an audit entry.
func (p *Proxy) SetModeWithAudit(mode Mode, user string) {
	p.mu.Lock()
	oldMode := p.mode
	p.mode = mode
	p.mu.Unlock()

	p.audit.RecordModeChange(user, oldMode, mode, "")
}

// SetDomainModeWithAudit sets a domain override and records an audit entry.
func (p *Proxy) SetDomainModeWithAudit(domain string, mode Mode, user string) {
	p.mu.Lock()
	oldMode := p.GetModeForDomainLocked(domain)
	if p.cfg.DomainOverrides == nil {
		p.cfg.DomainOverrides = make(map[string]Mode)
	}
	p.cfg.DomainOverrides[domain] = mode
	p.mu.Unlock()

	p.audit.RecordModeChange(user, oldMode, mode, domain)
}

// GetModeForDomainLocked returns effective mode while already holding lock.
// Caller must hold p.mu.
func (p *Proxy) GetModeForDomainLocked(domain string) Mode {
	if override, ok := p.cfg.DomainOverrides[domain]; ok {
		return override
	}
	return p.mode
}

// GetAuditLog returns all audit entries.
func (p *Proxy) GetAuditLog() []ModeChangeAudit {
	return p.audit.GetEntries()
}
```

**Step 5.6: Run test to verify it passes**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run TestProxy_AuditIntegration -v`
Expected: PASS

**Step 5.7: Run linter and all tests**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run && go test ./...`
Expected: All pass

**Step 5.8: Commit**

```bash
git add nc-http-proxy/proxy.go nc-http-proxy/proxy_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): integrate audit log with proxy

Adds SetModeWithAudit and SetDomainModeWithAudit methods that
record mode changes to the audit log.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Add New Admin Endpoints to Proxy

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/admin.go`
- Modify: `/home/fsd42/dev/north-cloud/nc-http-proxy/admin_test.go`

**Step 6.1: Write failing test for domain mode endpoint**

Add to `admin_test.go`:

```go
func TestAdmin_SetDomainMode(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	admin := NewAdminHandler(proxy)

	// Set domain mode
	req := httptest.NewRequest(http.MethodPost, "/admin/mode/example.com/record", nil)
	req.Header.Set("X-User", "test-user")
	w := httptest.NewRecorder()

	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify override was set
	mode := proxy.GetModeForDomain("example.com")
	if mode != ModeRecord {
		t.Errorf("expected ModeRecord, got %s", mode)
	}
}

func TestAdmin_ClearDomainMode(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: map[string]Mode{"example.com": ModeRecord},
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodDelete, "/admin/mode/example.com", nil)
	w := httptest.NewRecorder()

	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify override was cleared
	mode := proxy.GetModeForDomain("example.com")
	if mode != ModeReplay {
		t.Errorf("expected ModeReplay (global), got %s", mode)
	}
}

func TestAdmin_GetAudit(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	// Record a mode change
	proxy.SetModeWithAudit(ModeRecord, "test-user")

	admin := NewAdminHandler(proxy)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	w := httptest.NewRecorder()

	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var entries []ModeChangeAudit
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(entries))
	}
}

func TestAdmin_SetHybridMode(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Mode:            ModeReplay,
		DomainOverrides: make(map[string]Mode),
		HybridFallback:  false,
		FixturesDir:     t.TempDir(),
		CacheDir:        t.TempDir(),
		CertsDir:        t.TempDir(),
	}

	proxy, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	admin := NewAdminHandler(proxy)

	// Enable hybrid mode
	req := httptest.NewRequest(http.MethodPost, "/admin/mode/hybrid", nil)
	w := httptest.NewRecorder()

	admin.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !proxy.HybridFallback() {
		t.Error("expected hybrid fallback to be enabled")
	}
}
```

**Step 6.2: Run tests to verify they fail**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run 'TestAdmin_SetDomainMode|TestAdmin_ClearDomainMode|TestAdmin_GetAudit|TestAdmin_SetHybridMode' -v`
Expected: FAIL - endpoints don't exist

**Step 6.3: Update admin.go ServeHTTP routing**

In `admin.go`, update `ServeHTTP` to add new routes. Find the switch statement and add:

```go
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	// ... existing cases ...

	// NEW: Domain mode override
	case strings.HasPrefix(path, "/admin/mode/") && r.Method == http.MethodPost:
		// Check if it's hybrid mode or domain mode
		remainder := strings.TrimPrefix(path, "/admin/mode/")
		if remainder == "hybrid" {
			h.handleSetHybrid(w, r)
			return
		}
		// Parse domain/mode from path: /admin/mode/{domain}/{mode}
		parts := strings.Split(remainder, "/")
		if len(parts) == 2 {
			h.handleSetDomainMode(w, r, parts[0], parts[1])
			return
		}
		// Fall through to existing mode handler if just /admin/mode/{mode}
		if len(parts) == 1 {
			h.handleModeSwitch(w, r)
			return
		}
		http.Error(w, "invalid path", http.StatusBadRequest)

	// NEW: Clear domain mode
	case strings.HasPrefix(path, "/admin/mode/") && r.Method == http.MethodDelete:
		domain := strings.TrimPrefix(path, "/admin/mode/")
		h.handleClearDomainMode(w, r, domain)

	// NEW: Audit log
	case path == "/admin/audit" && r.Method == http.MethodGet:
		h.handleGetAudit(w, r)

	// ... existing cases ...
	}
}
```

**Step 6.4: Implement new handler methods**

In `admin.go`, add:

```go
func (h *AdminHandler) handleSetDomainMode(w http.ResponseWriter, r *http.Request, domain, modeStr string) {
	mode := Mode(modeStr)
	if !isValidMode(mode) {
		h.writeError(w, http.StatusBadRequest, "invalid_mode", "mode must be replay, record, or live")
		return
	}

	domain = safePath(domain)
	if domain == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_domain", "domain is required")
		return
	}

	user := r.Header.Get("X-User")
	if user == "" {
		user = "anonymous"
	}

	h.proxy.SetDomainModeWithAudit(domain, mode, user)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"domain": domain,
		"mode":   mode,
	})
}

func (h *AdminHandler) handleClearDomainMode(w http.ResponseWriter, r *http.Request, domain string) {
	domain = safePath(domain)
	if domain == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_domain", "domain is required")
		return
	}

	h.proxy.ClearDomainMode(domain)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"domain":  domain,
		"cleared": true,
	})
}

func (h *AdminHandler) handleGetAudit(w http.ResponseWriter, _ *http.Request) {
	entries := h.proxy.GetAuditLog()
	h.writeJSON(w, http.StatusOK, entries)
}

func (h *AdminHandler) handleSetHybrid(w http.ResponseWriter, r *http.Request) {
	// Toggle based on query param or body, default to enable
	enable := true
	if r.URL.Query().Get("enable") == "false" {
		enable = false
	}

	h.proxy.SetHybridFallback(enable)

	h.writeJSON(w, http.StatusOK, map[string]any{
		"hybrid_fallback": enable,
	})
}

func isValidMode(mode Mode) bool {
	return mode == ModeReplay || mode == ModeRecord || mode == ModeLive
}
```

**Step 6.5: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && go test -run 'TestAdmin_SetDomainMode|TestAdmin_ClearDomainMode|TestAdmin_GetAudit|TestAdmin_SetHybridMode' -v`
Expected: PASS

**Step 6.6: Update status response to include new fields**

In `admin.go`, update `handleStatus`:

```go
func (h *AdminHandler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	stats := h.proxy.cache.Stats()

	h.writeJSON(w, http.StatusOK, map[string]any{
		"mode":             h.proxy.Mode(),
		"fixtures_count":   stats.FixturesCount,
		"cache_count":      stats.CacheCount,
		"domains":          stats.Domains,
		"domain_overrides": h.proxy.GetDomainOverrides(), // NEW
		"hybrid_fallback":  h.proxy.HybridFallback(),     // NEW
	})
}
```

**Step 6.7: Run linter and all tests**

Run: `cd /home/fsd42/dev/north-cloud/nc-http-proxy && golangci-lint run && go test ./...`
Expected: All pass

**Step 6.8: Commit**

```bash
git add nc-http-proxy/admin.go nc-http-proxy/admin_test.go
git commit -m "$(cat <<'EOF'
feat(nc-http-proxy): add domain override, hybrid, and audit admin endpoints

New endpoints:
- POST /admin/mode/{domain}/{mode} - set per-domain override
- DELETE /admin/mode/{domain} - clear domain override
- GET /admin/audit - get mode change history
- POST /admin/mode/hybrid - enable/disable hybrid fallback

Status response now includes domain_overrides and hybrid_fallback.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Create ProxyClient for MCP Server

**Files:**
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/proxy.go`
- Create: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/client/proxy_test.go`

**Step 7.1: Write failing test for ProxyClient**

Create `proxy_test.go`:

```go
package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyClient_GetStatus(t *testing.T) {
	t.Helper()

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/status" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := map[string]any{
			"mode":             "replay",
			"fixtures_count":   10,
			"cache_count":      5,
			"domains":          []string{"example.com"},
			"domain_overrides": map[string]string{},
			"hybrid_fallback":  false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewProxyClient(server.URL)
	status, err := client.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Mode != "replay" {
		t.Errorf("expected mode 'replay', got %s", status.Mode)
	}
	if status.FixturesCount != 10 {
		t.Errorf("expected fixtures_count 10, got %d", status.FixturesCount)
	}
}

func TestProxyClient_SetMode(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/mode/record" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"mode": "record"})
	}))
	defer server.Close()

	client := NewProxyClient(server.URL)
	err := client.SetMode("record", "test-user")
	if err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
}
```

**Step 7.2: Run test to verify it fails**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestProxyClient -v`
Expected: FAIL - ProxyClient doesn't exist

**Step 7.3: Implement ProxyClient**

Create `proxy.go`:

```go
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	proxyRequestTimeout = 10 * time.Second
)

// ProxyStatus represents the proxy's current state.
type ProxyStatus struct {
	Mode            string            `json:"mode"`
	FixturesCount   int               `json:"fixtures_count"`
	CacheCount      int               `json:"cache_count"`
	Domains         []string          `json:"domains"`
	DomainOverrides map[string]string `json:"domain_overrides"`
	HybridFallback  bool              `json:"hybrid_fallback"`
}

// ModeChangeAudit represents a mode change event.
type ModeChangeAudit struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	OldMode   string `json:"old_mode"`
	NewMode   string `json:"new_mode"`
	Domain    string `json:"domain,omitempty"`
}

// CacheEntry represents a cached response.
type CacheEntry struct {
	Key       string `json:"key"`
	Method    string `json:"method"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Timestamp string `json:"timestamp"`
}

// ProxyClient communicates with the nc-http-proxy admin API.
type ProxyClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewProxyClient creates a new proxy client.
func NewProxyClient(baseURL string) *ProxyClient {
	return &ProxyClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: proxyRequestTimeout,
		},
	}
}

// GetStatus returns the proxy's current status.
func (c *ProxyClient) GetStatus() (*ProxyStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/admin/status", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var status ProxyStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &status, nil
}

// SetMode sets the global proxy mode.
func (c *ProxyClient) SetMode(mode, user string) error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/mode/%s", c.baseURL, mode)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if user != "" {
		req.Header.Set("X-User", user)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// SetDomainMode sets a per-domain mode override.
func (c *ProxyClient) SetDomainMode(domain, mode, user string) error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/mode/%s/%s", c.baseURL, domain, mode)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if user != "" {
		req.Header.Set("X-User", user)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// ClearDomainMode removes a domain mode override.
func (c *ProxyClient) ClearDomainMode(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/mode/%s", c.baseURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// SetHybridFallback enables or disables hybrid fallback mode.
func (c *ProxyClient) SetHybridFallback(enable bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/mode/hybrid?enable=%t", c.baseURL, enable)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// ListDomains returns all cached domains.
func (c *ProxyClient) ListDomains() ([]string, error) {
	status, err := c.GetStatus()
	if err != nil {
		return nil, err
	}
	return status.Domains, nil
}

// GetDomainCache returns cached entries for a domain.
func (c *ProxyClient) GetDomainCache(domain string) ([]CacheEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/cache/%s", c.baseURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var entries []CacheEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return entries, nil
}

// ClearCache clears all cached responses.
func (c *ProxyClient) ClearCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/admin/cache", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// ClearDomainCache clears cached responses for a specific domain.
func (c *ProxyClient) ClearDomainCache(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/admin/cache/%s", c.baseURL, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// GetAuditLog returns the mode change audit log.
func (c *ProxyClient) GetAuditLog() ([]ModeChangeAudit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), proxyRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/admin/audit", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var entries []ModeChangeAudit
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return entries, nil
}
```

**Step 7.4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./internal/client/... -run TestProxyClient -v`
Expected: PASS

**Step 7.5: Add more test coverage**

Add to `proxy_test.go`:

```go
func TestProxyClient_SetDomainMode(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/mode/example.com/record" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-User") != "test-user" {
			t.Errorf("expected X-User header 'test-user', got %s", r.Header.Get("X-User"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"domain": "example.com", "mode": "record"})
	}))
	defer server.Close()

	client := NewProxyClient(server.URL)
	err := client.SetDomainMode("example.com", "record", "test-user")
	if err != nil {
		t.Fatalf("SetDomainMode failed: %v", err)
	}
}

func TestProxyClient_ClearDomainMode(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/mode/example.com" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"domain": "example.com", "cleared": true})
	}))
	defer server.Close()

	client := NewProxyClient(server.URL)
	err := client.ClearDomainMode("example.com")
	if err != nil {
		t.Fatalf("ClearDomainMode failed: %v", err)
	}
}

func TestProxyClient_GetAuditLog(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/audit" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		entries := []map[string]string{
			{"timestamp": "2026-02-01T10:00:00Z", "user": "admin", "old_mode": "replay", "new_mode": "record"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	client := NewProxyClient(server.URL)
	entries, err := client.GetAuditLog()
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}
```

**Step 7.6: Run all tests and linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go test ./...`
Expected: All pass

**Step 7.7: Commit**

```bash
git add mcp-north-cloud/internal/client/proxy.go mcp-north-cloud/internal/client/proxy_test.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add ProxyClient for proxy admin API

Client provides methods for all proxy control operations:
- GetStatus, SetMode, SetDomainMode, ClearDomainMode
- SetHybridFallback, ListDomains, GetDomainCache
- ClearCache, ClearDomainCache, GetAuditLog

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Add ProxyURL to MCP Config

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/config/config.go`

**Step 8.1: Add ProxyURL to ServicesConfig**

In `config.go`, update `ServicesConfig`:

```go
type ServicesConfig struct {
	IndexManagerURL  string `env:"INDEX_MANAGER_URL"  yaml:"index_manager_url"`
	CrawlerURL       string `env:"CRAWLER_URL"        yaml:"crawler_url"`
	SourceManagerURL string `env:"SOURCE_MANAGER_URL" yaml:"source_manager_url"`
	PublisherURL     string `env:"PUBLISHER_URL"      yaml:"publisher_url"`
	SearchURL        string `env:"SEARCH_URL"         yaml:"search_url"`
	ClassifierURL    string `env:"CLASSIFIER_URL"     yaml:"classifier_url"`
	ProxyURL         string `env:"PROXY_URL"          yaml:"proxy_url"` // NEW
}
```

**Step 8.2: Add default value in NewDefault**

In `config.go`, update `NewDefault()`:

```go
func NewDefault() *Config {
	return &Config{
		Services: ServicesConfig{
			IndexManagerURL:  "http://localhost:8090",
			CrawlerURL:       "http://localhost:8060",
			SourceManagerURL: "http://localhost:8050",
			PublisherURL:     "http://localhost:8080",
			SearchURL:        "http://localhost:8090",
			ClassifierURL:    "http://localhost:8070",
			ProxyURL:         "http://localhost:8055", // NEW
		},
		// ... rest of defaults ...
	}
}
```

**Step 8.3: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 8.4: Commit**

```bash
git add mcp-north-cloud/internal/config/config.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): add ProxyURL to config

Default: http://localhost:8055
Env override: PROXY_URL

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Define Proxy Control Tools

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/tools.go`

**Step 9.1: Add getProxyControlTools function**

In `tools.go`, add before the `getAllTools` function:

```go
func getProxyControlTools() []Tool {
	return []Tool{
		{
			Name:        "get_proxy_status",
			Description: "Get the current status of the HTTP replay proxy including mode, domain overrides, and cache statistics. Use when: Checking proxy configuration before crawling.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "set_proxy_mode",
			Description: "Set the global proxy operating mode. Use when: Switching between replay (fixtures/cache), record (fetch + cache), live (pass-through), or hybrid (replay with record fallback) modes.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"mode": map[string]any{
						"type":        "string",
						"description": "Operating mode: 'replay' (read from cache), 'record' (fetch + cache), 'live' (pass-through), 'hybrid' (replay with record fallback)",
						"enum":        []string{"replay", "record", "live", "hybrid"},
					},
					"user": map[string]any{
						"type":        "string",
						"description": "User making the change (for audit log)",
					},
				},
				"required": []string{"mode"},
			},
		},
		{
			Name:        "set_domain_mode",
			Description: "Set a per-domain mode override. Use when: A specific domain needs different handling than the global mode (e.g., record mode for one domain while others use replay).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{
						"type":        "string",
						"description": "Domain to set override for (e.g., 'example.com')",
					},
					"mode": map[string]any{
						"type":        "string",
						"description": "Mode for this domain: 'replay', 'record', or 'live'",
						"enum":        []string{"replay", "record", "live"},
					},
					"user": map[string]any{
						"type":        "string",
						"description": "User making the change (for audit log)",
					},
				},
				"required": []string{"domain", "mode"},
			},
		},
		{
			Name:        "clear_domain_mode",
			Description: "Remove a per-domain mode override, reverting to global mode. Use when: A domain no longer needs special handling.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{
						"type":        "string",
						"description": "Domain to clear override for",
					},
				},
				"required": []string{"domain"},
			},
		},
		{
			Name:        "list_proxy_domains",
			Description: "List all domains with cached responses. Use when: Checking what domains have cached data available.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_domain_cache",
			Description: "View cached responses for a specific domain. Use when: Inspecting what responses are cached for a domain.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{
						"type":        "string",
						"description": "Domain to get cache entries for",
					},
				},
				"required": []string{"domain"},
			},
		},
		{
			Name:        "clear_proxy_cache",
			Description: "Clear all cached responses. Use when: Starting fresh or troubleshooting cache issues. WARNING: This clears the entire cache.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "clear_domain_cache",
			Description: "Clear cached responses for a specific domain. Use when: Refreshing cache for a single domain without affecting others.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{
						"type":        "string",
						"description": "Domain to clear cache for",
					},
				},
				"required": []string{"domain"},
			},
		},
		{
			Name:        "get_proxy_audit",
			Description: "Get the proxy mode change audit log. Use when: Reviewing who changed proxy modes and when.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
```

**Step 9.2: Update getAllTools to include proxy tools**

In `tools.go`, update `getAllTools()`:

```go
func getAllTools() []Tool {
	tools := make([]Tool, 0, toolGroupCount*estimatedToolsPerGroup)
	tools = append(tools, getWorkflowTools()...)
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	tools = append(tools, getDevelopmentTools()...)
	tools = append(tools, getProxyControlTools()...) // NEW
	return tools
}
```

**Step 9.3: Update toolGroupCount constant**

In `tools.go`, update the constant:

```go
const (
	toolGroupCount         = 9  // Was 8, now 9 with proxy tools
	estimatedToolsPerGroup = 5
)
```

**Step 9.4: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 9.5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/tools.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): define 9 proxy control tools

Tools: get_proxy_status, set_proxy_mode, set_domain_mode,
clear_domain_mode, list_proxy_domains, get_domain_cache,
clear_proxy_cache, clear_domain_cache, get_proxy_audit

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Register Tool Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/server.go`

**Step 10.1: Add proxyClient to Server struct**

In `server.go`, update the Server struct:

```go
type Server struct {
	crawlerClient      *client.CrawlerClient
	sourceManagerClient *client.SourceManagerClient
	publisherClient     *client.PublisherClient
	searchClient        *client.SearchClient
	classifierClient    *client.ClassifierClient
	indexManagerClient  *client.IndexManagerClient
	proxyClient         *client.ProxyClient // NEW
}
```

**Step 10.2: Update NewServer function**

In `server.go`, update `NewServer`:

```go
func NewServer(
	crawlerClient *client.CrawlerClient,
	sourceManagerClient *client.SourceManagerClient,
	publisherClient *client.PublisherClient,
	searchClient *client.SearchClient,
	classifierClient *client.ClassifierClient,
	indexManagerClient *client.IndexManagerClient,
	proxyClient *client.ProxyClient, // NEW
) *Server {
	return &Server{
		crawlerClient:       crawlerClient,
		sourceManagerClient: sourceManagerClient,
		publisherClient:     publisherClient,
		searchClient:        searchClient,
		classifierClient:    classifierClient,
		indexManagerClient:  indexManagerClient,
		proxyClient:         proxyClient, // NEW
	}
}
```

**Step 10.3: Add tool handlers to toolHandlers map**

In `server.go`, add to the `toolHandlers` map:

```go
var toolHandlers = map[string]toolHandlerFunc{
	// ... existing handlers ...

	// Proxy control tools
	"get_proxy_status":   (*Server).handleGetProxyStatus,
	"set_proxy_mode":     (*Server).handleSetProxyMode,
	"set_domain_mode":    (*Server).handleSetDomainMode,
	"clear_domain_mode":  (*Server).handleClearDomainMode,
	"list_proxy_domains": (*Server).handleListProxyDomains,
	"get_domain_cache":   (*Server).handleGetDomainCache,
	"clear_proxy_cache":  (*Server).handleClearProxyCache,
	"clear_domain_cache": (*Server).handleClearDomainCache,
	"get_proxy_audit":    (*Server).handleGetProxyAudit,
}
```

**Step 10.4: Run linter (expect errors - handlers don't exist yet)**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: Errors about undefined handler methods (we'll fix in next task)

**Step 10.5: Commit partial progress**

```bash
git add mcp-north-cloud/internal/mcp/server.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): register proxy client and tool handlers

Adds proxyClient to Server struct and registers 9 handler functions
in toolHandlers map. Handler implementations follow in next commit.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Implement Proxy Tool Handlers

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/internal/mcp/handlers.go`

**Step 11.1: Add handler argument structs**

In `handlers.go`, add at the appropriate location (with other args structs):

```go
// Proxy control argument structs
type setProxyModeArgs struct {
	Mode string `json:"mode"`
	User string `json:"user"`
}

type setDomainModeArgs struct {
	Domain string `json:"domain"`
	Mode   string `json:"mode"`
	User   string `json:"user"`
}

type clearDomainModeArgs struct {
	Domain string `json:"domain"`
}

type getDomainCacheArgs struct {
	Domain string `json:"domain"`
}

type clearDomainCacheArgs struct {
	Domain string `json:"domain"`
}
```

**Step 11.2: Implement handler methods**

In `handlers.go`, add the handler implementations:

```go
// handleGetProxyStatus returns the current proxy status.
func (s *Server) handleGetProxyStatus(id any, _ json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	status, err := s.proxyClient.GetStatus()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get proxy status: %v", err))
	}

	return s.successResponse(id, status)
}

// handleSetProxyMode sets the global proxy mode.
func (s *Server) handleSetProxyMode(id any, arguments json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	var args setProxyModeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.Mode == "" {
		return s.errorResponse(id, InvalidParams, "mode is required")
	}

	// Handle hybrid as a special case
	if args.Mode == "hybrid" {
		if err := s.proxyClient.SetHybridFallback(true); err != nil {
			return s.errorResponse(id, InternalError, fmt.Sprintf("failed to enable hybrid mode: %v", err))
		}
		return s.successResponse(id, map[string]any{
			"mode":            "hybrid",
			"hybrid_fallback": true,
		})
	}

	if err := s.proxyClient.SetMode(args.Mode, args.User); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to set proxy mode: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"mode": args.Mode,
	})
}

// handleSetDomainMode sets a per-domain mode override.
func (s *Server) handleSetDomainMode(id any, arguments json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	var args setDomainModeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.Domain == "" {
		return s.errorResponse(id, InvalidParams, "domain is required")
	}
	if args.Mode == "" {
		return s.errorResponse(id, InvalidParams, "mode is required")
	}

	if err := s.proxyClient.SetDomainMode(args.Domain, args.Mode, args.User); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to set domain mode: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"domain": args.Domain,
		"mode":   args.Mode,
	})
}

// handleClearDomainMode removes a domain mode override.
func (s *Server) handleClearDomainMode(id any, arguments json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	var args clearDomainModeArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.Domain == "" {
		return s.errorResponse(id, InvalidParams, "domain is required")
	}

	if err := s.proxyClient.ClearDomainMode(args.Domain); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to clear domain mode: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"domain":  args.Domain,
		"cleared": true,
	})
}

// handleListProxyDomains lists all cached domains.
func (s *Server) handleListProxyDomains(id any, _ json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	domains, err := s.proxyClient.ListDomains()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to list domains: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"domains": domains,
		"count":   len(domains),
	})
}

// handleGetDomainCache returns cached entries for a domain.
func (s *Server) handleGetDomainCache(id any, arguments json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	var args getDomainCacheArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.Domain == "" {
		return s.errorResponse(id, InvalidParams, "domain is required")
	}

	entries, err := s.proxyClient.GetDomainCache(args.Domain)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get domain cache: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"domain":  args.Domain,
		"entries": entries,
		"count":   len(entries),
	})
}

// handleClearProxyCache clears all cached responses.
func (s *Server) handleClearProxyCache(id any, _ json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	if err := s.proxyClient.ClearCache(); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to clear cache: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"cleared": true,
	})
}

// handleClearDomainCache clears cached responses for a domain.
func (s *Server) handleClearDomainCache(id any, arguments json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	var args clearDomainCacheArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}

	if args.Domain == "" {
		return s.errorResponse(id, InvalidParams, "domain is required")
	}

	if err := s.proxyClient.ClearDomainCache(args.Domain); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to clear domain cache: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"domain":  args.Domain,
		"cleared": true,
	})
}

// handleGetProxyAudit returns the mode change audit log.
func (s *Server) handleGetProxyAudit(id any, _ json.RawMessage) *Response {
	if s.proxyClient == nil {
		return s.errorResponse(id, InvalidParams, "proxy client not configured")
	}

	entries, err := s.proxyClient.GetAuditLog()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("failed to get audit log: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}
```

**Step 11.3: Run linter**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run`
Expected: No errors

**Step 11.4: Run all tests**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && go test ./...`
Expected: All pass

**Step 11.5: Commit**

```bash
git add mcp-north-cloud/internal/mcp/handlers.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): implement 9 proxy control tool handlers

Handlers for: get_proxy_status, set_proxy_mode, set_domain_mode,
clear_domain_mode, list_proxy_domains, get_domain_cache,
clear_proxy_cache, clear_domain_cache, get_proxy_audit

Each handler validates inputs, calls ProxyClient, and returns
structured JSON responses.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Initialize Proxy Client in main.go

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/main.go`

**Step 12.1: Add import and initialize client**

In `main.go`, add the proxy client initialization after other clients:

```go
// Create proxy client
proxyClient := client.NewProxyClient(cfg.Services.ProxyURL)
```

**Step 12.2: Update NewServer call**

In `main.go`, update the server creation:

```go
server := mcp.NewServer(
	crawlerClient,
	sourceManagerClient,
	publisherClient,
	searchClient,
	classifierClient,
	indexManagerClient,
	proxyClient, // NEW
)
```

**Step 12.3: Run linter and build**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && golangci-lint run && go build -o bin/mcp-north-cloud .`
Expected: No errors, binary builds successfully

**Step 12.4: Commit**

```bash
git add mcp-north-cloud/main.go
git commit -m "$(cat <<'EOF'
feat(mcp-north-cloud): initialize proxy client in main

Adds ProxyClient creation and passes to Server constructor.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Integration Testing

**Files:**
- No file changes - verification only

**Step 13.1: Start proxy service**

Run: `task proxy:up`
Expected: Proxy starts on port 8055

**Step 13.2: Verify proxy status endpoint**

Run: `curl -s http://localhost:8055/admin/status | jq`
Expected: JSON with mode, fixtures_count, cache_count, domains, domain_overrides, hybrid_fallback

**Step 13.3: Build and test MCP server**

Run: `cd /home/fsd42/dev/north-cloud/mcp-north-cloud && task build`
Expected: Build succeeds

**Step 13.4: Test get_proxy_status tool**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_proxy_status","arguments":{}}}' | ./bin/mcp-north-cloud`
Expected: JSON response with proxy status

**Step 13.5: Test set_proxy_mode tool**

Run: `echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"set_proxy_mode","arguments":{"mode":"record","user":"test"}}}' | ./bin/mcp-north-cloud`
Expected: Success response

**Step 13.6: Test get_proxy_audit tool**

Run: `echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_proxy_audit","arguments":{}}}' | ./bin/mcp-north-cloud`
Expected: Audit log with recent mode change

**Step 13.7: Document verification results**

Create a brief note of what worked and any issues found.

---

## Task 14: Update Documentation

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/mcp-north-cloud/README.md`

**Step 14.1: Add proxy tools section**

Add a new section documenting the proxy control tools:

```markdown
## Proxy Control Tools (Dev-Only)

These tools control the nc-http-proxy service for development testing.

| Tool | Description |
|------|-------------|
| `get_proxy_status` | Get current proxy mode, overrides, and cache stats |
| `set_proxy_mode` | Set global mode (replay/record/live/hybrid) |
| `set_domain_mode` | Set per-domain mode override |
| `clear_domain_mode` | Remove domain override |
| `list_proxy_domains` | List cached domains |
| `get_domain_cache` | View cached responses for domain |
| `clear_proxy_cache` | Clear all cache |
| `clear_domain_cache` | Clear specific domain cache |
| `get_proxy_audit` | Get mode change audit log |

### Configuration

Set `PROXY_URL` environment variable (default: `http://localhost:8055`).
```

**Step 14.2: Commit**

```bash
git add mcp-north-cloud/README.md
git commit -m "$(cat <<'EOF'
docs(mcp-north-cloud): document proxy control tools

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements Phase 1 with 14 tasks:

| Task | Description | New/Modified Files |
|------|-------------|-------------------|
| 1 | Add domain overrides to proxy config | nc-http-proxy/config.go |
| 2 | Add domain override methods to proxy | nc-http-proxy/proxy.go |
| 3 | Add hybrid fallback mode | nc-http-proxy/proxy.go |
| 4 | Create audit log | nc-http-proxy/audit.go |
| 5 | Integrate audit with proxy | nc-http-proxy/proxy.go, main.go |
| 6 | Add admin endpoints | nc-http-proxy/admin.go |
| 7 | Create ProxyClient | mcp-north-cloud/internal/client/proxy.go |
| 8 | Add ProxyURL config | mcp-north-cloud/internal/config/config.go |
| 9 | Define proxy tools | mcp-north-cloud/internal/mcp/tools.go |
| 10 | Register handlers | mcp-north-cloud/internal/mcp/server.go |
| 11 | Implement handlers | mcp-north-cloud/internal/mcp/handlers.go |
| 12 | Initialize in main | mcp-north-cloud/main.go |
| 13 | Integration testing | (verification only) |
| 14 | Update documentation | mcp-north-cloud/README.md |

**Total: ~14 commits, following TDD pattern throughout**

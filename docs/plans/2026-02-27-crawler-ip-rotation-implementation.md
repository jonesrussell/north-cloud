# Crawler IP Rotation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rotate outbound crawler traffic across multiple DigitalOcean Reserved IPs via Squid proxy to avoid IP bans and rate limits.

**Architecture:** A `doctl`-based management script provisions Reserved IPs and configures network interfaces. Squid on the host binds each IP to a dedicated port. A new `proxypool` Go package provides domain-sticky rotation for both Colly and frontier fetcher paths.

**Tech Stack:** Bash/doctl (infrastructure), Squid (proxy), Go (proxy pool package)

**Design Doc:** `docs/plans/2026-02-27-crawler-ip-rotation-design.md`

---

### Task 1: Infrastructure — IP Management Script

**Files:**
- Create: `scripts/manage-ips.sh`

**Step 1: Create the management script**

```bash
#!/usr/bin/env bash
set -euo pipefail

# manage-ips.sh — Provision and manage DigitalOcean Reserved IPs for crawler proxy rotation.
#
# Usage:
#   ./scripts/manage-ips.sh add --region sfo3
#   ./scripts/manage-ips.sh remove --ip 64.23.x.x
#   ./scripts/manage-ips.sh list
#   ./scripts/manage-ips.sh validate

INVENTORY_FILE="${INVENTORY_FILE:-/opt/north-cloud/proxy-ips.conf}"
SQUID_CONF="/etc/squid/squid.conf"
SQUID_CONF_TEMPLATE="/opt/north-cloud/squid.conf.template"
BASE_PORT=3128
DROPLET_ID=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

get_droplet_id() {
    if [[ -z "$DROPLET_ID" ]]; then
        DROPLET_ID=$(curl -s http://169.254.169.254/metadata/v1/id)
    fi
    echo "$DROPLET_ID"
}

get_droplet_region() {
    curl -s http://169.254.169.254/metadata/v1/region
}

ensure_inventory_file() {
    if [[ ! -f "$INVENTORY_FILE" ]]; then
        sudo mkdir -p "$(dirname "$INVENTORY_FILE")"
        sudo touch "$INVENTORY_FILE"
        sudo chmod 644 "$INVENTORY_FILE"
        log_info "Created inventory file: $INVENTORY_FILE"
    fi
}

cmd_add() {
    local region=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --region) region="$2"; shift 2 ;;
            *) log_error "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$region" ]]; then
        region=$(get_droplet_region)
        log_info "Using droplet region: $region"
    fi

    ensure_inventory_file

    local droplet_id
    droplet_id=$(get_droplet_id)

    log_info "Creating Reserved IP in region $region..."
    local ip
    ip=$(doctl compute reserved-ip create --region "$region" --output json | jq -r '.[0].ip')
    if [[ -z "$ip" || "$ip" == "null" ]]; then
        log_error "Failed to create Reserved IP"
        exit 1
    fi
    log_info "Created Reserved IP: $ip"

    log_info "Assigning $ip to droplet $droplet_id..."
    doctl compute reserved-ip-action assign "$ip" "$droplet_id" --wait
    log_info "Assigned $ip to droplet"

    # Get the anchor IP for this reserved IP
    log_info "Retrieving anchor IP..."
    local anchor_gateway
    anchor_gateway=$(curl -s http://169.254.169.254/metadata/v1/interfaces/public/0/anchor_ipv4/gateway)
    local anchor_ip
    anchor_ip=$(curl -s http://169.254.169.254/metadata/v1/interfaces/public/0/anchor_ipv4/address)

    log_info "Anchor IP: $anchor_ip, Gateway: $anchor_gateway"

    # Add the IP to the network interface
    if ! ip addr show dev eth0 | grep -q "$anchor_ip"; then
        log_info "Adding $anchor_ip to eth0..."
        sudo ip addr add "$anchor_ip/16" dev eth0
    else
        log_info "Anchor IP $anchor_ip already on eth0"
    fi

    # Add to inventory
    echo "$ip" | sudo tee -a "$INVENTORY_FILE" > /dev/null
    log_info "Added $ip to inventory"

    # Regenerate Squid config
    cmd_regenerate_squid

    # Print the new proxy URL
    local ip_count
    ip_count=$(wc -l < "$INVENTORY_FILE")
    local port=$((BASE_PORT + ip_count - 1))
    log_info "New proxy endpoint: http://172.17.0.1:$port"
    log_info "Add to CRAWLER_PROXY_POOL_URLS in .env and restart crawler"
}

cmd_remove() {
    local ip=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --ip) ip="$2"; shift 2 ;;
            *) log_error "Unknown option: $1"; exit 1 ;;
        esac
    done

    if [[ -z "$ip" ]]; then
        log_error "--ip is required"
        exit 1
    fi

    local droplet_id
    droplet_id=$(get_droplet_id)

    log_info "Unassigning $ip from droplet..."
    doctl compute reserved-ip-action unassign "$ip" --wait || true

    log_info "Deleting Reserved IP $ip..."
    doctl compute reserved-ip delete "$ip" --force

    # Remove from inventory
    sudo sed -i "/^${ip}$/d" "$INVENTORY_FILE"
    log_info "Removed $ip from inventory"

    # Regenerate Squid config
    cmd_regenerate_squid

    log_info "Removed $ip. Update CRAWLER_PROXY_POOL_URLS in .env and restart crawler"
}

cmd_list() {
    ensure_inventory_file

    echo "=== Inventory ($INVENTORY_FILE) ==="
    if [[ -s "$INVENTORY_FILE" ]]; then
        local idx=0
        while IFS= read -r ip; do
            local port=$((BASE_PORT + idx))
            echo "  $ip -> http://172.17.0.1:$port"
            idx=$((idx + 1))
        done < "$INVENTORY_FILE"
    else
        echo "  (empty)"
    fi

    echo ""
    echo "=== DigitalOcean Reserved IPs ==="
    doctl compute reserved-ip list --format IP,Region,DropletID --no-header
}

cmd_validate() {
    ensure_inventory_file

    local droplet_id
    droplet_id=$(get_droplet_id)
    local errors=0

    echo "=== Validation Report ==="
    echo ""

    # 1. Get DO Reserved IPs assigned to this droplet
    echo "--- DigitalOcean API ---"
    local do_ips
    do_ips=$(doctl compute reserved-ip list --format IP,DropletID --no-header | awk -v did="$droplet_id" '$2 == did {print $1}')
    if [[ -n "$do_ips" ]]; then
        echo "$do_ips" | while read -r ip; do echo "  Assigned: $ip"; done
    else
        echo "  No Reserved IPs assigned to this droplet"
    fi
    echo ""

    # 2. Check inventory file
    echo "--- Inventory File ($INVENTORY_FILE) ---"
    if [[ -s "$INVENTORY_FILE" ]]; then
        while IFS= read -r ip; do echo "  Listed: $ip"; done < "$INVENTORY_FILE"
    else
        echo "  (empty)"
    fi
    echo ""

    # 3. Check network interfaces
    echo "--- Network Interfaces (eth0) ---"
    ip addr show dev eth0 | grep "inet " | awk '{print "  Bound: " $2}'
    echo ""

    # 4. Compare: IPs in DO but not in inventory
    echo "--- Drift Detection ---"
    if [[ -n "$do_ips" ]]; then
        while IFS= read -r ip; do
            if ! grep -q "^${ip}$" "$INVENTORY_FILE" 2>/dev/null; then
                log_warn "IP $ip assigned in DO but NOT in inventory"
                errors=$((errors + 1))
            fi
        done <<< "$do_ips"
    fi

    # 5. Compare: IPs in inventory but not in DO
    if [[ -s "$INVENTORY_FILE" ]]; then
        while IFS= read -r ip; do
            if [[ -n "$do_ips" ]] && ! echo "$do_ips" | grep -q "^${ip}$"; then
                log_warn "IP $ip in inventory but NOT assigned in DO"
                errors=$((errors + 1))
            fi
        done < "$INVENTORY_FILE"
    fi

    # 6. Check Squid config has the right number of ports
    if [[ -f "$SQUID_CONF" ]]; then
        local squid_ports
        squid_ports=$(grep -c "^http_port" "$SQUID_CONF" || echo 0)
        local inventory_count
        inventory_count=$(wc -l < "$INVENTORY_FILE" || echo 0)
        if [[ "$squid_ports" -ne "$inventory_count" ]]; then
            log_warn "Squid has $squid_ports ports but inventory has $inventory_count IPs"
            errors=$((errors + 1))
        else
            log_info "Squid port count matches inventory ($squid_ports)"
        fi
    fi

    echo ""
    if [[ "$errors" -eq 0 ]]; then
        log_info "No drift detected"
    else
        log_warn "$errors drift issue(s) found. Run 'add' or 'remove' to reconcile."
    fi
}

cmd_regenerate_squid() {
    ensure_inventory_file

    if [[ ! -s "$INVENTORY_FILE" ]]; then
        log_warn "Inventory is empty, skipping Squid config generation"
        return
    fi

    local tmpfile
    tmpfile=$(mktemp)

    # Generate header
    local inventory_hash
    inventory_hash=$(md5sum "$INVENTORY_FILE" | awk '{print $1}')
    cat > "$tmpfile" <<HEADER
# Auto-generated by manage-ips.sh — do not edit manually
# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Inventory hash: $inventory_hash
# IPs: $(wc -l < "$INVENTORY_FILE")

# Access control
acl localhost src 127.0.0.1/32
acl docker_bridge src 172.17.0.0/16

http_access allow localhost
http_access allow docker_bridge
http_access deny all

# Disable caching — pure forward proxy
cache deny all

HEADER

    # Generate port and ACL entries
    local idx=0
    while IFS= read -r ip; do
        local port=$((BASE_PORT + idx))
        echo "http_port $port" >> "$tmpfile"
        idx=$((idx + 1))
    done < "$INVENTORY_FILE"

    echo "" >> "$tmpfile"

    # Generate ACL and tcp_outgoing_address entries
    idx=0
    while IFS= read -r ip; do
        local port=$((BASE_PORT + idx))
        echo "acl port_${port} localport $port" >> "$tmpfile"
        idx=$((idx + 1))
    done < "$INVENTORY_FILE"

    echo "" >> "$tmpfile"

    idx=0
    local first_ip=""
    while IFS= read -r ip; do
        local port=$((BASE_PORT + idx))
        echo "tcp_outgoing_address $ip port_${port}" >> "$tmpfile"
        if [[ $idx -eq 0 ]]; then first_ip="$ip"; fi
        idx=$((idx + 1))
    done < "$INVENTORY_FILE"

    # Fallback rule
    echo "" >> "$tmpfile"
    echo "# Fallback for unmapped ports" >> "$tmpfile"
    echo "tcp_outgoing_address $first_ip" >> "$tmpfile"

    # Per-port access logging
    echo "" >> "$tmpfile"
    echo "# Per-port access logging" >> "$tmpfile"
    idx=0
    while IFS= read -r ip; do
        local port=$((BASE_PORT + idx))
        echo "access_log daemon:/var/log/squid/access.log squid port_${port}" >> "$tmpfile"
        idx=$((idx + 1))
    done < "$INVENTORY_FILE"

    # Validate before applying
    log_info "Validating generated Squid config..."
    if ! squid -k parse -f "$tmpfile" 2>/dev/null; then
        log_error "Generated Squid config is invalid! Aborting."
        rm -f "$tmpfile"
        exit 1
    fi

    # Atomic move
    sudo mv "$tmpfile" "$SQUID_CONF"
    log_info "Squid config written to $SQUID_CONF"

    # Reload
    sudo squid -k reconfigure
    log_info "Squid reloaded"
}

# Main dispatcher
case "${1:-help}" in
    add)      shift; cmd_add "$@" ;;
    remove)   shift; cmd_remove "$@" ;;
    list)     cmd_list ;;
    validate) cmd_validate ;;
    regenerate-squid) cmd_regenerate_squid ;;
    help|*)
        echo "Usage: $0 {add|remove|list|validate|regenerate-squid}"
        echo ""
        echo "Commands:"
        echo "  add --region <region>   Create and assign a new Reserved IP"
        echo "  remove --ip <ip>        Unassign and delete a Reserved IP"
        echo "  list                    Show current IPs and proxy endpoints"
        echo "  validate                Check consistency between DO, inventory, and Squid"
        echo "  regenerate-squid        Rebuild squid.conf from inventory"
        ;;
esac
```

**Step 2: Make the script executable and verify syntax**

Run: `chmod +x scripts/manage-ips.sh && bash -n scripts/manage-ips.sh`
Expected: no output (valid syntax)

**Step 3: Commit**

```bash
git add scripts/manage-ips.sh
git commit -m "feat(infra): add IP management script for crawler proxy rotation"
```

---

### Task 2: Proxy Pool Package — Core Types and Tests

**Files:**
- Create: `crawler/internal/proxypool/pool.go`
- Create: `crawler/internal/proxypool/pool_test.go`

**Step 1: Write the failing tests**

```go
// crawler/internal/proxypool/pool_test.go
package proxypool_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/north-cloud/crawler/internal/proxypool"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", raw, err)
	}
	return u
}

func TestPool_RoundRobinAssignment(t *testing.T) {
	t.Helper() // remove — this is a test func not a helper
	urls := []string{"http://localhost:3128", "http://localhost:3129", "http://localhost:3130"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	// First domain gets first proxy
	p1, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p1.String() != urls[0] {
		t.Errorf("expected %s, got %s", urls[0], p1)
	}

	// Second domain gets second proxy (round-robin)
	p2, err := pool.ProxyFor("other.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p2.String() != urls[1] {
		t.Errorf("expected %s, got %s", urls[1], p2)
	}

	// Third domain gets third proxy
	p3, err := pool.ProxyFor("third.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p3.String() != urls[2] {
		t.Errorf("expected %s, got %s", urls[2], p3)
	}
}

func TestPool_DomainSticky(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	// Assign proxy to domain
	p1, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same domain gets same proxy (sticky)
	p2, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.String() != p2.String() {
		t.Errorf("expected sticky proxy %s, got %s", p1, p2)
	}
}

func TestPool_StickyTTLExpiry(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(1*time.Millisecond))

	p1, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	// After TTL, domain may get reassigned via round-robin
	p2, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The important thing is it doesn't panic and returns a valid proxy
	if p2 == nil {
		t.Error("expected non-nil proxy after TTL expiry")
	}
	_ = p1 // p1 may or may not equal p2 depending on round-robin state
}

func TestPool_MarkUnhealthy(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129"}
	pool := proxypool.New(urls,
		proxypool.WithStickyTTL(10*time.Minute),
		proxypool.WithHealthBackoff(1*time.Hour),
	)

	// Mark first proxy unhealthy
	pool.MarkUnhealthy(mustParseURL(t, urls[0]))

	// New domain should skip the unhealthy proxy
	p, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.String() != urls[1] {
		t.Errorf("expected healthy proxy %s, got %s", urls[1], p)
	}
}

func TestPool_AllUnhealthy(t *testing.T) {
	urls := []string{"http://localhost:3128"}
	pool := proxypool.New(urls,
		proxypool.WithStickyTTL(10*time.Minute),
		proxypool.WithHealthBackoff(1*time.Hour),
	)

	pool.MarkUnhealthy(mustParseURL(t, urls[0]))

	// When all proxies are unhealthy, still return one (best-effort)
	p, err := pool.ProxyFor("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Error("expected non-nil proxy even when all unhealthy")
	}
}

func TestPool_EmptyURLs(t *testing.T) {
	pool := proxypool.New(nil, proxypool.WithStickyTTL(10*time.Minute))

	_, err := pool.ProxyFor("example.com")
	if err == nil {
		t.Error("expected error for empty proxy pool")
	}
}

func TestPool_ConcurrentAccess(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129", "http://localhost:3130"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	const goroutines = 50
	errs := make(chan error, goroutines)

	for i := range goroutines {
		go func(i int) {
			domain := "domain" + string(rune('a'+i%26)) + ".com"
			_, err := pool.ProxyFor(domain)
			errs <- err
		}(i)
	}

	for range goroutines {
		if err := <-errs; err != nil {
			t.Errorf("concurrent access error: %v", err)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd crawler && go test ./internal/proxypool/... -v`
Expected: compilation error — package does not exist yet

**Step 3: Write the implementation**

```go
// crawler/internal/proxypool/pool.go
package proxypool

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoProxies is returned when the pool has no proxy URLs configured.
var ErrNoProxies = errors.New("proxy pool has no URLs configured")

// Default configuration values.
const (
	DefaultStickyTTL     = 10 * time.Minute
	DefaultHealthBackoff = 5 * time.Minute
)

// Option configures a Pool.
type Option func(*Pool)

// WithStickyTTL sets the domain-sticky TTL duration.
func WithStickyTTL(d time.Duration) Option {
	return func(p *Pool) { p.stickyTTL = d }
}

// WithHealthBackoff sets the unhealthy backoff duration.
func WithHealthBackoff(d time.Duration) Option {
	return func(p *Pool) { p.healthBackoff = d }
}

type domainEntry struct {
	proxy      *url.URL
	assignedAt time.Time
}

// Pool provides domain-sticky proxy rotation across a set of proxy URLs.
// It is safe for concurrent use.
type Pool struct {
	proxies       []*url.URL
	stickyTTL     time.Duration
	healthBackoff time.Duration

	mu       sync.RWMutex
	domains  map[string]domainEntry
	health   map[string]time.Time // proxy URL string -> unhealthy until
	robinIdx atomic.Uint64
}

// New creates a Pool from the given proxy URL strings.
func New(urls []string, opts ...Option) *Pool {
	parsed := make([]*url.URL, 0, len(urls))
	for _, raw := range urls {
		if u, err := url.Parse(raw); err == nil {
			parsed = append(parsed, u)
		}
	}

	p := &Pool{
		proxies:       parsed,
		stickyTTL:     DefaultStickyTTL,
		healthBackoff: DefaultHealthBackoff,
		domains:       make(map[string]domainEntry),
		health:        make(map[string]time.Time),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// ProxyFor returns the proxy URL to use for the given domain.
// If the domain has a sticky assignment within the TTL, that proxy is returned.
// Otherwise, the next healthy proxy is assigned via round-robin.
func (p *Pool) ProxyFor(domain string) (*url.URL, error) {
	if len(p.proxies) == 0 {
		return nil, ErrNoProxies
	}

	now := time.Now()

	// Check for existing sticky assignment
	p.mu.RLock()
	entry, exists := p.domains[domain]
	p.mu.RUnlock()

	if exists && now.Sub(entry.assignedAt) < p.stickyTTL {
		return entry.proxy, nil
	}

	// Assign a new proxy via round-robin, preferring healthy ones
	proxy := p.nextHealthy(now)

	p.mu.Lock()
	p.domains[domain] = domainEntry{proxy: proxy, assignedAt: now}
	p.mu.Unlock()

	return proxy, nil
}

// nextHealthy returns the next healthy proxy via round-robin.
// If all proxies are unhealthy, returns the next one anyway (best-effort).
func (p *Pool) nextHealthy(now time.Time) *url.URL {
	proxyCount := uint64(len(p.proxies))

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try each proxy in round-robin order, looking for a healthy one
	startIdx := p.robinIdx.Add(1) - 1
	for i := range proxyCount {
		candidate := p.proxies[(startIdx+i)%proxyCount]
		unhealthyUntil, isUnhealthy := p.health[candidate.String()]
		if !isUnhealthy || now.After(unhealthyUntil) {
			return candidate
		}
	}

	// All unhealthy — return next in rotation (best-effort)
	return p.proxies[startIdx%proxyCount]
}

// MarkUnhealthy marks a proxy as unhealthy for the configured backoff duration.
func (p *Pool) MarkUnhealthy(proxy *url.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health[proxy.String()] = time.Now().Add(p.healthBackoff)
}

// URLs returns the list of proxy URL strings (for logging/diagnostics).
func (p *Pool) URLs() []string {
	result := make([]string, len(p.proxies))
	for i, u := range p.proxies {
		result[i] = u.String()
	}
	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `cd crawler && go test ./internal/proxypool/... -v`
Expected: all tests PASS

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/proxypool/...`
Expected: no violations

**Step 6: Commit**

```bash
git add crawler/internal/proxypool/
git commit -m "feat(crawler): add domain-sticky proxy pool package"
```

---

### Task 3: Proxy Pool — HTTP ProxyFunc Adapter

**Files:**
- Modify: `crawler/internal/proxypool/pool.go`
- Create: `crawler/internal/proxypool/transport.go`
- Create: `crawler/internal/proxypool/transport_test.go`

**Step 1: Write the failing tests**

```go
// crawler/internal/proxypool/transport_test.go
package proxypool_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/north-cloud/crawler/internal/proxypool"
)

func TestProxyFunc_RoutesToCorrectProxy(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	proxyFunc := pool.ProxyFunc()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/page", http.NoBody)
	proxyURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxyURL == nil {
		t.Fatal("expected non-nil proxy URL")
	}

	// Same domain should return same proxy (sticky)
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com/other", http.NoBody)
	proxyURL2, err := proxyFunc(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxyURL.String() != proxyURL2.String() {
		t.Errorf("expected sticky proxy %s, got %s", proxyURL, proxyURL2)
	}
}

func TestProxyFunc_DifferentDomainsGetDifferentProxies(t *testing.T) {
	urls := []string{"http://localhost:3128", "http://localhost:3129"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	proxyFunc := pool.ProxyFunc()

	req1, _ := http.NewRequest(http.MethodGet, "http://first.com/page", http.NoBody)
	p1, err := proxyFunc(req1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req2, _ := http.NewRequest(http.MethodGet, "http://second.com/page", http.NoBody)
	p2, err := proxyFunc(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p1.String() == p2.String() {
		t.Errorf("expected different proxies for different domains, both got %s", p1)
	}
}

func TestNewTransport_SetsProxyFunc(t *testing.T) {
	urls := []string{"http://localhost:3128"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	transport := proxypool.NewTransport(pool, nil)
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set")
	}
}

func TestNewTransport_PreservesBaseSettings(t *testing.T) {
	urls := []string{"http://localhost:3128"}
	pool := proxypool.New(urls, proxypool.WithStickyTTL(10*time.Minute))

	base := &http.Transport{
		MaxIdleConns:    200,
		IdleConnTimeout: 45 * time.Second,
	}

	transport := proxypool.NewTransport(pool, base)
	if transport.MaxIdleConns != 200 {
		t.Errorf("expected MaxIdleConns=200, got %d", transport.MaxIdleConns)
	}
	if transport.IdleConnTimeout != 45*time.Second {
		t.Errorf("expected IdleConnTimeout=45s, got %v", transport.IdleConnTimeout)
	}
	if transport.Proxy == nil {
		t.Error("expected Proxy function to be set on cloned transport")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd crawler && go test ./internal/proxypool/... -v -run TestProxy`
Expected: compilation error — functions don't exist yet

**Step 3: Write the implementation**

```go
// crawler/internal/proxypool/transport.go
package proxypool

import (
	"net/http"
	"net/url"
)

// ProxyFunc returns an http.ProxyFunc that routes requests through the pool
// based on the request's target domain. Compatible with both http.Transport.Proxy
// and colly.SetProxyFunc (which accepts func(*http.Request) (*url.URL, error)).
func (p *Pool) ProxyFunc() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		return p.ProxyFor(req.URL.Hostname())
	}
}

// NewTransport creates an *http.Transport with the pool's ProxyFunc set.
// If base is non-nil, its settings are cloned and the Proxy field is overridden.
// If base is nil, a default transport is created.
func NewTransport(pool *Pool, base *http.Transport) *http.Transport {
	var t *http.Transport
	if base != nil {
		t = base.Clone()
	} else {
		t = &http.Transport{}
	}
	t.Proxy = pool.ProxyFunc()
	return t
}
```

**Step 4: Run tests to verify they pass**

Run: `cd crawler && go test ./internal/proxypool/... -v`
Expected: all tests PASS

**Step 5: Run linter**

Run: `cd crawler && golangci-lint run ./internal/proxypool/...`
Expected: no violations

**Step 6: Commit**

```bash
git add crawler/internal/proxypool/
git commit -m "feat(crawler): add ProxyFunc and Transport adapters for proxy pool"
```

---

### Task 4: Config — Add Proxy Pool Fields

**Files:**
- Modify: `crawler/internal/config/crawler/config.go:97-103` (add fields after existing proxy config)
- Modify: `crawler/internal/config/crawler/config.go:171-172` (add defaults)

**Step 1: Add the new config fields**

In the `Config` struct (after line 100, the existing `ProxyURLs` field), add:

```go
	// ProxyPoolEnabled enables domain-sticky proxy pool rotation (replaces ProxiesEnabled)
	ProxyPoolEnabled bool `env:"CRAWLER_PROXY_POOL_ENABLED" yaml:"proxy_pool_enabled"`
	// ProxyPoolURLs is the list of Squid proxy endpoints for the pool (replaces ProxyURLs)
	ProxyPoolURLs []string `env:"CRAWLER_PROXY_POOL_URLS" yaml:"proxy_pool_urls"`
	// ProxyStickyTTL is how long a domain stays assigned to the same proxy
	ProxyStickyTTL time.Duration `env:"CRAWLER_PROXY_STICKY_TTL" yaml:"proxy_sticky_ttl"`
```

In the `New()` defaults (after line 172), add:

```go
		ProxyPoolEnabled:           false,
		ProxyPoolURLs:              nil,
		ProxyStickyTTL:             10 * time.Minute,
```

**Step 2: Run existing tests to verify nothing breaks**

Run: `cd crawler && go test ./internal/config/... -v`
Expected: all existing tests PASS

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run ./internal/config/...`
Expected: no violations

**Step 4: Commit**

```bash
git add crawler/internal/config/crawler/config.go
git commit -m "feat(crawler): add proxy pool config fields"
```

---

### Task 5: Integration — Wire Proxy Pool into Colly Collector

**Files:**
- Modify: `crawler/internal/crawler/collector.go:252-270` (replace `setupProxyRotation`)

**Step 1: Update setupProxyRotation to use the proxy pool**

Replace the `setupProxyRotation` method (lines 252-270) with:

```go
// setupProxyRotation configures proxy rotation if enabled.
// Supports both the legacy round-robin mode and the new domain-sticky proxy pool.
func (c *Crawler) setupProxyRotation() error {
	// New proxy pool takes priority
	if c.cfg.ProxyPoolEnabled && len(c.cfg.ProxyPoolURLs) > 0 {
		pool := proxypool.New(c.cfg.ProxyPoolURLs,
			proxypool.WithStickyTTL(c.cfg.ProxyStickyTTL),
		)
		c.collector.SetProxyFunc(pool.ProxyFunc())
		c.GetJobLogger().Info(logs.CategoryLifecycle,
			"Domain-sticky proxy pool enabled",
			logs.Int("proxy_count", len(c.cfg.ProxyPoolURLs)),
			logs.Duration("sticky_ttl", c.cfg.ProxyStickyTTL),
		)
		return nil
	}

	// Legacy round-robin fallback
	if !c.cfg.ProxiesEnabled || len(c.cfg.ProxyURLs) == 0 {
		return nil
	}

	rp, err := proxy.RoundRobinProxySwitcher(c.cfg.ProxyURLs...)
	if err != nil {
		return fmt.Errorf("failed to create proxy switcher: %w", err)
	}

	c.collector.SetProxyFunc(rp)

	c.GetJobLogger().Info(logs.CategoryLifecycle,
		"Proxy rotation enabled",
		logs.Int("proxy_count", len(c.cfg.ProxyURLs)),
	)
	return nil
}
```

Add the import for `proxypool` at the top of `collector.go`:

```go
"github.com/north-cloud/crawler/internal/proxypool"
```

**Step 2: Run tests**

Run: `cd crawler && go test ./internal/crawler/... -v`
Expected: all tests PASS

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run ./internal/crawler/...`
Expected: no violations

**Step 4: Commit**

```bash
git add crawler/internal/crawler/collector.go
git commit -m "feat(crawler): integrate proxy pool into Colly collector"
```

---

### Task 6: Integration — Wire Proxy Pool into Frontier Fetcher

**Files:**
- Modify: `crawler/internal/bootstrap/services.go:567-595` (add proxy transport to frontier HTTP client)

**Step 1: Update createFrontierWorkerPool to use proxy pool**

In `createFrontierWorkerPool` (around lines 567-570 where `httpClient` is created), wrap the transport with the proxy pool:

```go
	var httpTransport *http.Transport
	crawlerCfg := deps.Config.GetCrawlerConfig()
	if crawlerCfg.ProxyPoolEnabled && len(crawlerCfg.ProxyPoolURLs) > 0 {
		pool := proxypool.New(crawlerCfg.ProxyPoolURLs,
			proxypool.WithStickyTTL(crawlerCfg.ProxyStickyTTL),
		)
		httpTransport = proxypool.NewTransport(pool, nil)
		deps.Logger.Info("Frontier fetcher proxy pool enabled",
			infralogger.Int("proxy_count", len(crawlerCfg.ProxyPoolURLs)))
	}

	httpClient := &http.Client{
		Timeout:       fetcherCfg.RequestTimeout,
		CheckRedirect: checkRedirect,
	}
	if httpTransport != nil {
		httpClient.Transport = httpTransport
	}
```

Add the import for `proxypool`:

```go
"github.com/north-cloud/crawler/internal/proxypool"
```

**Step 2: Run tests**

Run: `cd crawler && go test ./internal/bootstrap/... -v`
Expected: all tests PASS (or no test files — that's fine)

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run ./internal/bootstrap/...`
Expected: no violations

**Step 4: Commit**

```bash
git add crawler/internal/bootstrap/services.go
git commit -m "feat(crawler): integrate proxy pool into frontier fetcher"
```

---

### Task 7: Integration — Wire Proxy Pool into Feed Poller

**Files:**
- Modify: `crawler/internal/bootstrap/services.go:396` (first feed poller creation)
- Modify: `crawler/internal/bootstrap/services.go:495` (second feed poller creation)

**Step 1: Update feed poller HTTP client creation**

Both lines creating the feed HTTP fetcher need updating. The pattern is the same — wrap the HTTP client with a proxied transport. Since both the frontier (Task 6) and feed poller share the same bootstrap file, extract a helper:

At the bottom of `services.go` (before the `logAdapter` type), add:

```go
// buildProxiedHTTPClient creates an HTTP client with proxy pool transport if enabled.
func buildProxiedHTTPClient(crawlerCfg *crawlerconfig.Config, timeout time.Duration, log infralogger.Logger) *http.Client {
	client := &http.Client{Timeout: timeout}
	if crawlerCfg.ProxyPoolEnabled && len(crawlerCfg.ProxyPoolURLs) > 0 {
		pool := proxypool.New(crawlerCfg.ProxyPoolURLs,
			proxypool.WithStickyTTL(crawlerCfg.ProxyStickyTTL),
		)
		client.Transport = proxypool.NewTransport(pool, nil)
		log.Info("HTTP client proxy pool enabled",
			infralogger.Int("proxy_count", len(crawlerCfg.ProxyPoolURLs)))
	}
	return client
}
```

Then update line 396:

```go
	httpFetcher := feed.NewHTTPFetcher(buildProxiedHTTPClient(
		deps.Config.GetCrawlerConfig(), feedHTTPFetchTimeout, deps.Logger,
	))
```

And line 495 (the second feed poller creation):

```go
	httpFetcher := feed.NewHTTPFetcher(buildProxiedHTTPClient(
		deps.Config.GetCrawlerConfig(), feedHTTPFetchTimeout, deps.Logger,
	))
```

Also refactor the frontier fetcher from Task 6 to use this helper (but keep the `CheckRedirect` override).

**Step 2: Run tests**

Run: `cd crawler && go test ./... -count=1`
Expected: all tests PASS

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run`
Expected: no violations

**Step 4: Commit**

```bash
git add crawler/internal/bootstrap/services.go
git commit -m "feat(crawler): integrate proxy pool into feed poller"
```

---

### Task 8: Environment Configuration

**Files:**
- Modify: `docker-compose.prod.yml` (add proxy pool env vars to crawler service)
- Modify: `.env.example` (add proxy pool env vars)

**Step 1: Add env vars to docker-compose.prod.yml**

In the crawler service's environment section, add:

```yaml
      - CRAWLER_PROXY_POOL_ENABLED=${CRAWLER_PROXY_POOL_ENABLED:-false}
      - CRAWLER_PROXY_POOL_URLS=${CRAWLER_PROXY_POOL_URLS:-}
      - CRAWLER_PROXY_STICKY_TTL=${CRAWLER_PROXY_STICKY_TTL:-10m}
```

**Step 2: Add env vars to .env.example**

Add a section:

```bash
# Proxy Pool (IP rotation via Squid)
CRAWLER_PROXY_POOL_ENABLED=false
CRAWLER_PROXY_POOL_URLS=
CRAWLER_PROXY_STICKY_TTL=10m
```

**Step 3: Commit**

```bash
git add docker-compose.prod.yml .env.example
git commit -m "feat(crawler): add proxy pool environment configuration"
```

---

### Task 9: End-to-End Verification

**Step 1: Run all crawler tests**

Run: `cd crawler && go test ./... -count=1 -race`
Expected: all tests PASS, no race conditions

**Step 2: Run full linter**

Run: `task lint:crawler`
Expected: no violations

**Step 3: Verify the build compiles**

Run: `cd crawler && go build -o /dev/null .`
Expected: compiles successfully

**Step 4: Commit any remaining fixes**

If any fixes were needed, commit them.

---

### Task 10: Production Deployment (Manual — On Server)

This task is executed manually on the production server (`jones@northcloud.one`).

**Step 1: Install Squid**

```bash
ssh jones@northcloud.one
sudo apt update && sudo apt install -y squid
sudo systemctl stop squid  # Stop until configured
```

**Step 2: Deploy the management script**

```bash
cd /opt/north-cloud
git pull origin main  # or the feature branch
chmod +x scripts/manage-ips.sh
```

**Step 3: Add Reserved IPs**

```bash
./scripts/manage-ips.sh add --region tor1  # or sfo3, nyc3, etc.
./scripts/manage-ips.sh add --region tor1
# Optionally: ./scripts/manage-ips.sh add --region tor1
```

**Step 4: Verify**

```bash
./scripts/manage-ips.sh list
./scripts/manage-ips.sh validate
```

**Step 5: Update .env**

```bash
# Edit .env on the server
CRAWLER_PROXY_POOL_ENABLED=true
CRAWLER_PROXY_POOL_URLS=http://172.17.0.1:3128,http://172.17.0.1:3129
CRAWLER_PROXY_STICKY_TTL=10m
```

**Step 6: Restart crawler**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build crawler
```

**Step 7: Verify traffic is routing through proxies**

```bash
# Check Squid access logs
sudo tail -f /var/log/squid/access.log

# Check crawler logs for proxy info
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f crawler | grep proxy
```

# Crawler IP Rotation Design

**Date:** 2026-02-27
**Goal:** Avoid IP bans and rate limits by rotating outbound crawler traffic across multiple DigitalOcean Reserved IPs via Squid proxy.

---

## Architecture Overview

Three layers, each with a single responsibility:

1. **Infrastructure** — `manage-ips.sh` provisions Reserved IPs via `doctl`, configures network interfaces, maintains an inventory file
2. **Proxy** — Squid on the host binds each IP to a dedicated port, the crawler connects via Docker gateway
3. **Crawler** — A new `proxypool` package provides domain-sticky rotation for both Colly and frontier fetcher

```
[Crawler Container]
   |
   | http://172.17.0.1:3128  (IP A)
   | http://172.17.0.1:3129  (IP B)
   | http://172.17.0.1:3130  (IP C)
   v
[Squid on Host]
   |
   | tcp_outgoing_address per port
   v
[Target Sites]
```

---

## Section 1: Infrastructure Layer

### Management Script (`scripts/manage-ips.sh`)

Commands:

- **`add --region <region>`** — Creates a Reserved IP via `doctl compute reserved-ip create`, assigns it to the droplet, retrieves the anchor IP from the DO metadata API, configures the network interface via `ip addr add`, persists via netplan, updates the inventory file, regenerates Squid config, reloads Squid.
- **`remove --ip <ip>`** — Reverse of add: removes interface, unassigns and releases the Reserved IP, updates inventory, regenerates config, reloads.
- **`list`** — Shows current IPs and their status (from inventory file).
- **`validate`** — Queries DO API for all Reserved IPs assigned to this droplet, compares against `/opt/north-cloud/proxy-ips.conf`, compares against `ip addr show`, compares against Squid config. Reports drift but does not auto-fix.

### Inventory File (`/opt/north-cloud/proxy-ips.conf`)

One IP per line. Read by the Squid config generator.

### Network Configuration

- Each Reserved IP gets its anchor IP from `169.254.169.254/metadata/v1/...`
- Added as secondary addresses on `eth0`
- Persisted via netplan (Ubuntu) so they survive reboots
- Cloud-init network config disabled (`/etc/cloud/cloud.cfg.d/99-disable-network-config.cfg`)
- The droplet's original IP stays as the default route (SSH, API, Caddy traffic)

---

## Section 2: Proxy Layer (Squid)

### Installation & Management

- Runs as a Docker container (`ubuntu/squid:latest`) with `network_mode: host`
- Managed via docker-compose alongside other services
- `network_mode: host` gives Squid direct access to the host's network interfaces for outbound IP binding
- Config mounted from `/opt/north-cloud/squid/squid.conf`, logs from `/opt/north-cloud/squid/logs/`

### Port-per-IP Model

Each `http_port` maps to one outbound IP:

```squid
http_port 3128
http_port 3129

acl port_3128 localport 3128
acl port_3129 localport 3129

tcp_outgoing_address 64.23.x.x port_3128
tcp_outgoing_address 64.23.x.y port_3129

# Fallback for unmapped ports — deterministic behavior
tcp_outgoing_address 64.23.x.x
```

Base port: `3128`, each subsequent IP increments by 1.

### Config Generation

- Script reads `/opt/north-cloud/proxy-ips.conf` and generates `squid.conf`
- Generates into a temp file, then atomically moves into place (avoids partial writes)
- Validates with `squid -k parse` before reloading (prevents breaking the running instance)
- Includes a header comment with timestamp and inventory hash for drift debugging
- Reloads via `squid -k reconfigure` (zero-downtime)

### Access Control

```squid
acl localhost src 127.0.0.1/32
acl docker_bridge src 172.17.0.0/16

http_access allow localhost
http_access allow docker_bridge
http_access deny all
```

- No caching (pure forward proxy for IP binding)
- No authentication (localhost-only access)

### Logging

Per-port access log tags for easy debugging:

```squid
access_log /var/log/squid/access.log squid port_3128
access_log /var/log/squid/access.log squid port_3129
```

### Docker Integration

Crawler reaches Squid via Docker gateway IP (`172.17.0.1`), not `host.docker.internal` (unreliable on Linux). Proxy URLs:

```
http://172.17.0.1:3128
http://172.17.0.1:3129
```

---

## Section 3: Crawler Changes

### New Package: `crawler/internal/proxypool/`

Replaces the current `RoundRobinProxySwitcher` (Colly-only) with a shared proxy pool.

#### Domain-Sticky Rotation

- Map of `domain -> {proxyURL, assignedAt}` tracks IP-to-domain assignments
- When requesting a proxy for a domain:
  1. If domain has a sticky assignment within the TTL window, return that proxy
  2. Otherwise, round-robin assign the next proxy, record it
- Stale entries cleaned up lazily on access
- Thread-safe via `sync.RWMutex`

#### Integration Points

1. **Colly path** — Custom `ProxyFunc` via `SetProxyFunc(func(*http.Request) (*url.URL, error))` that calls the domain-sticky pool
2. **Frontier fetcher** — Custom `http.Transport` with `Proxy` function calling the same pool. Injected via `WorkerPoolConfig`'s optional `http.Client`
3. **Feed poller** — Same pattern: inject a proxied `http.Client`

#### Health Awareness (Reactive)

- Connection errors (not HTTP errors) mark a proxy unhealthy for a backoff period
- Unhealthy proxies skipped during assignment
- No active health checking — keeps it simple

### Configuration (env vars)

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_PROXY_POOL_ENABLED` | `false` | Feature toggle (replaces `CRAWLER_PROXIES_ENABLED`) |
| `CRAWLER_PROXY_POOL_URLS` | — | Comma-separated Squid endpoints (replaces `CRAWLER_PROXY_URLS`) |
| `CRAWLER_PROXY_STICKY_TTL` | `10m` | Domain sticky duration |

---

## Section 4: Deployment & Operations

### Operator Workflow

**Adding an IP:**
```bash
./scripts/manage-ips.sh add --region sfo3
# Update .env with new proxy URL
# Restart crawler container
```

**Removing an IP:**
```bash
./scripts/manage-ips.sh remove --ip 64.23.x.x
# Update .env, restart crawler
```

**Validating state:**
```bash
./scripts/manage-ips.sh validate
# Reports drift between DO API, inventory, interfaces, Squid config
```

### Cost

- Reserved IPs: free when attached, $5/mo if floating unattached
- 2-3 IPs to start = $0 ongoing
- Squid overhead: negligible

### What Stays the Same

- Droplet's original IP remains the default route (SSH, API, Caddy/nginx)
- Docker networking unchanged
- No changes to other services
- Crawler's existing `proxy` field in response logs continues working

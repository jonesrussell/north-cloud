# Proxy Observability: Ship Squid Logs to Loki

**Date:** 2026-02-27
**Status:** Design

## Problem

proxy-nyc1 (NYC dedicated proxy droplet) runs native Squid and generates access/cache logs at `/var/log/squid/`. These logs are not shipped to the Loki/Grafana observability stack on razor-crest (Toronto). The local squid Docker container on razor-crest also writes file-based logs that aren't explicitly tailed by Alloy.

## Architecture

### Components

```
proxy-nyc1 (NYC1)                          razor-crest (TOR1)
┌──────────────────────┐                   ┌──────────────────────────────┐
│  Squid (native)      │                   │  Docker                      │
│  └─ /var/log/squid/  │                   │  ├─ Loki (:3100)             │
│      ├─ access.log   │                   │  ├─ Alloy                    │
│      └─ cache.log    │                   │  │   └─ tails local squid    │
│                      │    HTTP push      │  │       logs too             │
│  Alloy (native)      │ ──────────────►   │  ├─ Grafana                  │
│  └─ tails squid logs │  :3100            │  └─ squid container          │
│     pushes to Loki   │                   │      └─ /opt/north-cloud/    │
└──────────────────────┘                   │          squid/logs/          │
                                           └──────────────────────────────┘
Firewall: only 67.205.164.249 → port 3100
```

### Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Log shipper on proxy-nyc1 | Alloy (native install) | Consistent tooling. ~50-80MB RAM is workable on 512MB+swap. No Docker (same reason as Squid). |
| Loki connectivity | Public port 3100, firewalled to one IP | Simple. No TLS/auth overhead. Single IP allowlist via iptables DOCKER-USER chain. |
| Log processing | Minimal — raw lines with labels | Parse/dashboard later. Ship with `service`, `host`, `log_type`, `project` labels. |
| Local squid on razor-crest | Also wire up via Alloy file tailing | Squid writes to files, not stdout. Alloy already runs — just add config + bind mount. |

### Future: Dual-VPC Isolation

DigitalOcean VPCs are region-scoped. Each droplet should eventually live in its own regional VPC:

- `north-cloud-tor1-vpc` (razor-crest, 10.124.0.0/20)
- `north-cloud-nyc1-vpc` (proxy-nyc1, 10.125.0.0/20)

This requires droplet recreation (VPC assignment is creation-time only). The logging design works identically with or without VPCs — Alloy pushes to razor-crest's public IP either way. VPC isolation is an additive security layer to pursue separately.

---

## Part 1: Alloy on proxy-nyc1

### Installation

Install Alloy via apt (Grafana's official repo). This sets up a systemd service automatically.

```bash
# Add Grafana apt repo
apt-get install -y gpg
mkdir -p /etc/apt/keyrings/
curl -fsSL https://apt.grafana.com/gpg.key | gpg --dearmor -o /etc/apt/keyrings/grafana.gpg
echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" \
  > /etc/apt/sources.list.d/grafana.list
apt-get update
apt-get install -y alloy
```

### File permissions

Squid logs are `proxy:proxy 640`. Add the `alloy` user to the `proxy` group:

```bash
usermod -aG proxy alloy
systemctl restart alloy
```

### Configuration

File: `/etc/alloy/config.alloy`

```hcl
logging {
  level  = "info"
  format = "logfmt"
}

local.file_match "squid_access" {
  path_targets = [{
    __path__ = "/var/log/squid/access.log",
    project  = "north-cloud",
    service  = "squid-proxy",
    host     = "proxy-nyc1",
    log_type = "access",
    job      = "file",
  }]
  sync_period = "10s"
}

local.file_match "squid_cache" {
  path_targets = [{
    __path__ = "/var/log/squid/cache.log",
    project  = "north-cloud",
    service  = "squid-proxy",
    host     = "proxy-nyc1",
    log_type = "cache",
    job      = "file",
  }]
  sync_period = "10s"
}

loki.source.file "squid_access" {
  targets       = local.file_match.squid_access.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}

loki.source.file "squid_cache" {
  targets       = local.file_match.squid_cache.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}

loki.write "north_cloud" {
  endpoint {
    url                 = "http://147.182.150.145:3100/loki/api/v1/push"
    remote_timeout      = "10s"
    batch_wait          = "1s"
    batch_size          = "100KiB"
    min_backoff_period  = "500ms"
    max_backoff_period  = "5m"
    max_backoff_retries = 10
  }
}
```

### Systemd

The apt package creates `/lib/systemd/system/alloy.service`. Enable and start:

```bash
systemctl enable alloy
systemctl start alloy
systemctl status alloy
journalctl -u alloy -f  # verify no errors
```

---

## Part 2: Expose Loki on razor-crest

### Docker Compose

In `docker-compose.prod.yml`, add port mapping to the Loki service:

```yaml
loki:
  restart: always
  ports:
    - "3100:3100"  # Exposed for remote Alloy ingestion (firewalled)
  volumes:
    - ./infrastructure/loki/loki-config.prod.yml:/etc/loki/config.yml:ro
    - loki_data:/loki
  environment:
    LOKI_LOG_LEVEL: info
```

### Firewall

Docker bypasses UFW, so we use the `DOCKER-USER` iptables chain. Allow only proxy-nyc1's IP:

```bash
# Allow proxy-nyc1 to reach Loki
sudo iptables -I DOCKER-USER -s 67.205.164.249 -p tcp --dport 3100 -j ACCEPT

# Drop all other external access to Loki
sudo iptables -I DOCKER-USER 2 -p tcp --dport 3100 -j DROP
```

Make rules persistent across reboots:

```bash
sudo apt-get install -y iptables-persistent
sudo netfilter-persistent save
```

---

## Part 3: Local squid logs on razor-crest

The local squid Docker container writes to `/opt/north-cloud/squid/logs/` (bind-mounted from the container). Alloy auto-discovers the container via Docker socket but only gets stderr — file-based access/cache logs need explicit tailing.

### Docker Compose

Add squid log directory as a bind mount to the Alloy container in `docker-compose.prod.yml`:

```yaml
alloy:
  restart: always
  volumes:
    - ./infrastructure/alloy/config.alloy:/etc/alloy/config.alloy:ro
    - /var/run/docker.sock:/var/run/docker.sock:ro
    - alloy_data:/var/lib/alloy
    - /opt/north-cloud/squid/logs:/mnt/squid-logs:ro          # NEW
    # Deployer sites...
    - /home/deployer/streetcode-laravel:/mnt/sites/streetcode-laravel:ro
    - /home/deployer/orewire-laravel:/mnt/sites/orewire-laravel:ro
    - /home/deployer/coforge:/mnt/sites/coforge:ro
    - /home/deployer/movies-of-war.com:/mnt/sites/movies-of-war.com:ro
```

### Alloy config

Add to `infrastructure/alloy/config.alloy` (before the `loki.process` sections):

```hcl
// ── Local Squid proxy logs (razor-crest Docker container) ─────────────────
local.file_match "squid_local_access" {
  path_targets = [{
    __path__ = "/mnt/squid-logs/access.log",
    project  = "north-cloud",
    service  = "squid-proxy",
    host     = "razor-crest",
    log_type = "access",
    job      = "file",
  }]
  sync_period = "10s"
}

local.file_match "squid_local_cache" {
  path_targets = [{
    __path__ = "/mnt/squid-logs/cache.log",
    project  = "north-cloud",
    service  = "squid-proxy",
    host     = "razor-crest",
    log_type = "cache",
    job      = "file",
  }]
  sync_period = "10s"
}

loki.source.file "squid_local_access" {
  targets       = local.file_match.squid_local_access.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}

loki.source.file "squid_local_cache" {
  targets       = local.file_match.squid_local_cache.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}
```

---

## Verification

After deployment, confirm logs flow end-to-end:

```bash
# On proxy-nyc1: check Alloy is running and tailing
systemctl status alloy
journalctl -u alloy --no-pager -n 20

# On razor-crest: check Loki received logs
docker exec north-cloud-loki wget -qO- \
  'http://localhost:3100/loki/api/v1/query?query={service="squid-proxy"}' | head

# In Grafana: Explore → Loki
# Query: {service="squid-proxy"}
# Should see logs from both host=proxy-nyc1 and host=razor-crest
```

---

## Rollback

- **proxy-nyc1**: `systemctl stop alloy && apt-get remove alloy`
- **razor-crest Loki port**: Remove `ports` from docker-compose, redeploy, remove iptables rules
- **razor-crest Alloy config**: Revert config.alloy changes, remove squid-logs bind mount, redeploy

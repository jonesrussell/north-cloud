# Proxy Observability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship Squid proxy logs from proxy-nyc1 (NYC) and local razor-crest into Loki/Grafana.

**Architecture:** Alloy installed natively on proxy-nyc1 tails Squid log files and pushes to Loki on razor-crest over HTTP (public IP, firewalled to one source IP). Local squid logs on razor-crest are tailed by the existing Alloy Docker container via a new bind mount. See `docs/plans/2026-02-27-proxy-observability-design.md` for full design.

**Tech Stack:** Grafana Alloy, Loki, Squid, iptables (DOCKER-USER chain), systemd

---

### Task 1: Firewall — Allow proxy-nyc1 to reach Loki on razor-crest

Loki port 3100 is already exposed in `docker-compose.base.yml` (line 510: `"${LOKI_PORT:-3100}:3100"`). The `ufw-docker` rules in the DOCKER-USER iptables chain block all external access by default. We need to punch a hole for proxy-nyc1's IP only.

**Files:**
- None (remote server iptables configuration)

**Step 1: Verify Loki is reachable internally on razor-crest**

Run on razor-crest:
```bash
ssh jones@northcloud.one "docker exec north-cloud-loki-1 wget -qO- http://localhost:3100/ready"
```
Expected: `ready`

**Step 2: Verify Loki is blocked externally (before firewall change)**

Run from proxy-nyc1:
```bash
ssh root@proxy-nyc1.northcloud.one "curl -s --connect-timeout 5 http://147.182.150.145:3100/ready || echo 'BLOCKED (expected)'"
```
Expected: `BLOCKED (expected)` — confirming ufw-docker blocks it.

**Step 3: Add iptables rule to allow proxy-nyc1**

Insert rule at position 11 in DOCKER-USER chain (after port 80/443 allows, before ufw-docker-logging-deny):
```bash
ssh jones@northcloud.one "sudo iptables -I DOCKER-USER 11 -s 67.205.164.249 -p tcp --dport 3100 -j RETURN"
```
Expected: No output (success).

**Step 4: Verify the rule is in place**

```bash
ssh jones@northcloud.one "sudo iptables -L DOCKER-USER -n --line-numbers | grep 3100"
```
Expected: Line showing `RETURN tcp -- 67.205.164.249 0.0.0.0/0 tcp dpt:3100`

**Step 5: Verify Loki is now reachable from proxy-nyc1**

```bash
ssh root@proxy-nyc1.northcloud.one "curl -s --connect-timeout 5 http://147.182.150.145:3100/ready"
```
Expected: `ready`

**Step 6: Persist the iptables rule**

The DOCKER-USER chain is managed by ufw-docker. Add the rule to `/etc/ufw/after.rules` so it survives reboots and `ufw reload`:

```bash
ssh jones@northcloud.one "sudo cat /etc/ufw/after.rules | grep -c 'DOCKER-USER' | head -5"
```

Look for the `*filter` section that contains DOCKER-USER rules. Add the Loki rule there. The exact edit depends on the file contents — insert before any `-A DOCKER-USER ... -j DROP` or logging-deny lines:

```
-A DOCKER-USER -s 67.205.164.249/32 -p tcp --dport 3100 -j RETURN
```

Then verify persistence:
```bash
ssh jones@northcloud.one "sudo ufw reload && sudo iptables -L DOCKER-USER -n | grep 3100"
```
Expected: Rule still present after reload.

**Step 7: Commit** (nothing to commit yet — this is server config, not repo files)

---

### Task 2: Install Alloy on proxy-nyc1

**Files:**
- None (remote server package installation)

**Step 1: Add Grafana apt repository**

```bash
ssh root@proxy-nyc1.northcloud.one "apt-get install -y gpg && mkdir -p /etc/apt/keyrings/ && curl -fsSL https://apt.grafana.com/gpg.key | gpg --dearmor -o /etc/apt/keyrings/grafana.gpg && echo 'deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main' > /etc/apt/sources.list.d/grafana.list && apt-get update"
```
Expected: Package lists updated, no errors.

**Step 2: Install Alloy**

```bash
ssh root@proxy-nyc1.northcloud.one "apt-get install -y alloy"
```
Expected: Alloy installed. The apt package creates a systemd unit at `/lib/systemd/system/alloy.service`.

**Step 3: Verify installation**

```bash
ssh root@proxy-nyc1.northcloud.one "alloy --version && systemctl status alloy --no-pager | head -5"
```
Expected: Version output and service status (may be inactive/dead until configured).

**Step 4: Grant Alloy read access to Squid logs**

Squid logs are `proxy:proxy 640`. Add the `alloy` user to the `proxy` group:
```bash
ssh root@proxy-nyc1.northcloud.one "usermod -aG proxy alloy && id alloy"
```
Expected: `alloy` user shows `proxy` in its groups.

---

### Task 3: Configure and start Alloy on proxy-nyc1

**Files:**
- Create (remote): `/etc/alloy/config.alloy`

**Step 1: Write the Alloy configuration**

```bash
ssh root@proxy-nyc1.northcloud.one "cat > /etc/alloy/config.alloy << 'ALLOYEOF'
logging {
  level  = \"info\"
  format = \"logfmt\"
}

local.file_match \"squid_access\" {
  path_targets = [{
    __path__ = \"/var/log/squid/access.log\",
    project  = \"north-cloud\",
    service  = \"squid-proxy\",
    host     = \"proxy-nyc1\",
    log_type = \"access\",
    job      = \"file\",
  }]
  sync_period = \"10s\"
}

local.file_match \"squid_cache\" {
  path_targets = [{
    __path__ = \"/var/log/squid/cache.log\",
    project  = \"north-cloud\",
    service  = \"squid-proxy\",
    host     = \"proxy-nyc1\",
    log_type = \"cache\",
    job      = \"file\",
  }]
  sync_period = \"10s\"
}

loki.source.file \"squid_access\" {
  targets       = local.file_match.squid_access.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}

loki.source.file \"squid_cache\" {
  targets       = local.file_match.squid_cache.targets
  forward_to    = [loki.write.north_cloud.receiver]
  tail_from_end = true
}

loki.write \"north_cloud\" {
  endpoint {
    url                 = \"http://147.182.150.145:3100/loki/api/v1/push\"
    remote_timeout      = \"10s\"
    batch_wait          = \"1s\"
    batch_size          = \"100KiB\"
    min_backoff_period  = \"500ms\"
    max_backoff_period  = \"5m\"
    max_backoff_retries = 10
  }
}
ALLOYEOF"
```

**Step 2: Verify config file was written**

```bash
ssh root@proxy-nyc1.northcloud.one "cat /etc/alloy/config.alloy"
```
Expected: Config contents as above, valid HCL.

**Step 3: Check Alloy's systemd unit environment file**

The apt package may use `/etc/default/alloy` for CLI args. Verify it points to our config:
```bash
ssh root@proxy-nyc1.northcloud.one "cat /etc/default/alloy 2>/dev/null || echo 'no defaults file'"
```
If it exists, ensure it contains the config path. If not, Alloy's systemd unit should default to `/etc/alloy/config.alloy`.

**Step 4: Enable and start Alloy**

```bash
ssh root@proxy-nyc1.northcloud.one "systemctl enable alloy && systemctl restart alloy"
```
Expected: No errors.

**Step 5: Verify Alloy is running and tailing logs**

```bash
ssh root@proxy-nyc1.northcloud.one "systemctl status alloy --no-pager && echo '---' && journalctl -u alloy --no-pager -n 20"
```
Expected: `active (running)`. Journal should show Alloy starting, discovering targets, and writing to Loki. No errors about file permissions or Loki connectivity.

**Step 6: Verify Alloy memory usage is acceptable**

```bash
ssh root@proxy-nyc1.northcloud.one "ps aux | grep alloy | grep -v grep && echo '---' && free -m"
```
Expected: Alloy RSS under ~100MB. Total system memory usage should leave headroom.

---

### Task 4: Wire up local squid logs on razor-crest

The local squid Docker container writes access/cache logs to `/opt/north-cloud/squid/logs/` (bind-mounted from container). Alloy discovers the container via Docker socket but Squid writes logs to files, not stdout. Add file tailing.

**Files:**
- Modify: `docker-compose.prod.yml` (Alloy volumes section, ~line 644)
- Modify: `infrastructure/alloy/config.alloy` (add squid log tailing, after line 226)

**Step 1: Add squid log bind mount to Alloy in docker-compose.prod.yml**

In `docker-compose.prod.yml`, add the squid logs bind mount to the `alloy` service volumes:

```yaml
  alloy:
    restart: always
    volumes:
      - ./infrastructure/alloy/config.alloy:/etc/alloy/config.alloy:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - alloy_data:/var/lib/alloy
      - /opt/north-cloud/squid/logs:/mnt/squid-logs:ro          # Local squid proxy logs
      # Deployer sites - mount site roots so current/ symlink resolves across deploys
      - /home/deployer/streetcode-laravel:/mnt/sites/streetcode-laravel:ro
      - /home/deployer/orewire-laravel:/mnt/sites/orewire-laravel:ro
      - /home/deployer/coforge:/mnt/sites/coforge:ro
      - /home/deployer/movies-of-war.com:/mnt/sites/movies-of-war.com:ro
    environment:
      ALLOY_LOG_LEVEL: info
```

The new line is: `- /opt/north-cloud/squid/logs:/mnt/squid-logs:ro`

**Step 2: Add squid log tailing to Alloy config**

In `infrastructure/alloy/config.alloy`, add after the movies_of_war_logs section (after line 226) and before the Laravel log processing section (line 228):

```hcl
// ── Local Squid proxy logs (razor-crest Docker container) ─────────────────
// Squid writes access/cache logs to files, not stdout.
// Bind-mounted from /opt/north-cloud/squid/logs/ into /mnt/squid-logs/.

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

**Step 3: Verify the config changes look correct**

Read both files and confirm the edits are syntactically correct.

**Step 4: Commit the local changes**

```bash
git add docker-compose.prod.yml infrastructure/alloy/config.alloy
git commit -m "feat(observability): add squid proxy log tailing to Alloy

Wire up local razor-crest squid logs via bind mount and file tailing.
Also supports remote proxy-nyc1 logs pushed via Alloy native install."
```

---

### Task 5: Deploy and verify end-to-end

**Files:**
- None (deployment commands)

**Step 1: Push changes to remote**

```bash
git push -u origin main
```

**Step 2: Deploy on razor-crest**

Pull latest and redeploy Alloy (and Loki if needed):
```bash
ssh jones@northcloud.one "cd /opt/north-cloud && git pull && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build alloy"
```
Expected: Alloy container recreated with new config and squid log mount.

**Step 3: Verify local squid logs flowing to Loki**

```bash
ssh jones@northcloud.one "docker exec north-cloud-loki-1 wget -qO- 'http://localhost:3100/loki/api/v1/query?query={service=\"squid-proxy\",host=\"razor-crest\"}' 2>/dev/null | head -5"
```
Expected: JSON response with log entries.

**Step 4: Verify remote proxy-nyc1 logs flowing to Loki**

```bash
ssh jones@northcloud.one "docker exec north-cloud-loki-1 wget -qO- 'http://localhost:3100/loki/api/v1/query?query={service=\"squid-proxy\",host=\"proxy-nyc1\"}' 2>/dev/null | head -5"
```
Expected: JSON response with log entries from proxy-nyc1.

**Step 5: Verify in Grafana**

Open Grafana (https://northcloud.one/grafana/) → Explore → Loki data source.

Queries to test:
- `{service="squid-proxy"}` — all proxy logs from both hosts
- `{service="squid-proxy", host="proxy-nyc1"}` — remote proxy only
- `{service="squid-proxy", host="razor-crest"}` — local proxy only
- `{service="squid-proxy", log_type="access"}` — access logs only
- `{service="squid-proxy", log_type="cache"}` — cache logs only

Expected: Logs from both hosts, both log types.

**Step 6: Verify Alloy memory on proxy-nyc1 is stable**

```bash
ssh root@proxy-nyc1.northcloud.one "free -m && echo '---' && ps aux --sort=-rss | head -5"
```
Expected: System not swapping excessively. Alloy RSS stable under ~100MB.

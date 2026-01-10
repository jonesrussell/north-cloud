# Promtail to Grafana Alloy Migration Guide

## Executive Summary

This document guides the migration from Promtail to Grafana Alloy for the North Cloud logging infrastructure.

**Critical Timeline:** Promtail reaches End-of-Life (EOL) on **March 2, 2026**. After this date:
- ❌ No security patches
- ❌ No bug fixes
- ❌ No feature updates

**Migration Status:** ✅ Configuration complete, ready for testing

## Why Migrate to Alloy?

### Promtail EOL Timeline
- **February 13, 2025:** Promtail enters Long-Term Support (LTS)
- **February 28, 2026:** Commercial support ends
- **March 2, 2026:** Promtail End-of-Life (EOL)

### Alloy Advantages
- ✅ **Future-proof:** Active development and long-term support
- ✅ **Vendor-neutral:** Supports multiple telemetry backends (Loki, Tempo, Prometheus, etc.)
- ✅ **Unified collector:** Single agent for logs, metrics, and traces
- ✅ **Better performance:** More efficient resource usage
- ✅ **Modern architecture:** Component-based configuration in HCL format
- ✅ **Enhanced observability:** Built-in debugging UI at http://localhost:12345

## Migration Approach

We're using a **phased migration** approach:

### Phase 1: Parallel Operation (Current)
- Both Promtail and Alloy run simultaneously
- Both send logs to Loki (duplicate writes, acceptable during migration)
- Allows validation that Alloy collects logs identically to Promtail
- Grafana queries work with logs from either source

### Phase 2: Validation (1 week)
- Monitor Alloy logs and performance
- Verify all North Cloud services' logs are captured
- Test Grafana queries and dashboards
- Compare log volumes between Promtail and Alloy

### Phase 3: Cutover
- Stop Promtail service
- Remove Promtail from docker-compose files
- Keep Promtail configuration archived for rollback if needed

## Configuration Comparison

### Promtail Configuration (YAML)
**File:** `/infrastructure/promtail/promtail-config.yml`
- Format: YAML (196 lines)
- Style: Declarative scrape_configs
- Pipeline: Linear stages

### Alloy Configuration (HCL)
**File:** `/infrastructure/alloy/config.alloy`
- Format: HCL - HashiCorp Configuration Language
- Style: Component-based (discovery → relabel → source → process → write)
- Pipeline: Composable components with explicit data flow

### Key Differences

| Feature | Promtail | Alloy |
|---------|----------|-------|
| **Config Format** | YAML | HCL (HashiCorp Configuration Language) |
| **Position File** | `/tmp/positions/positions.yaml` | `/var/lib/alloy/data/loki.source.docker.<name>/positions.yml` |
| **Web UI Port** | 9080 | 12345 |
| **Metrics Prefix** | `promtail_*` | `alloy_*` |
| **Docker Discovery** | `docker_sd_configs` | `discovery.docker` component |
| **Relabeling** | `relabel_configs` | `discovery.relabel` component |
| **Pipeline Stages** | `pipeline_stages` | `loki.process` component |
| **Log Writing** | `clients` | `loki.write` component |

## Docker Compose Changes

### Services Added
```yaml
alloy:
  image: grafana/alloy:latest
  ports:
    - "12345:12345"  # Alloy UI/API
  volumes:
    - ./infrastructure/alloy/config.alloy:/etc/alloy/config.alloy:ro
    - /var/run/docker.sock:/var/run/docker.sock:ro
    - alloy_data:/var/lib/alloy
  depends_on:
    loki:
      condition: service_healthy
```

### Volumes Added
```yaml
volumes:
  alloy_data:  # Persistent storage for Alloy positions file
```

### Environment Variables Added
```bash
# .env.example
ALLOY_PORT=12345  # Alloy UI/API port
```

## Migration Steps

### Step 1: Add Alloy to .env file

Add to your `.env` file (copy from `.env.example`):
```bash
ALLOY_PORT=12345
```

### Step 2: Start Alloy alongside Promtail

```bash
# Start all services including both Promtail and Alloy
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Check Alloy is running
docker ps | grep alloy

# View Alloy logs
docker logs -f north-cloud-alloy
```

### Step 3: Verify Alloy UI

Open Alloy's debugging UI in your browser:
```
http://localhost:12345
```

You should see:
- ✅ Component graph showing data flow
- ✅ Discovery targets (Docker containers)
- ✅ Relabeling rules applied
- ✅ Log collection statistics

### Step 4: Validate Log Collection (1 week)

#### Check Both Collectors
```bash
# Promtail logs
docker logs north-cloud-promtail | grep "client.*push" | tail -5

# Alloy logs
docker logs north-cloud-alloy | grep "loki.write" | tail -5
```

#### Query Loki for Alloy-Collected Logs

In Grafana Explore (http://localhost:3000):

1. **Verify logs are arriving:**
   ```logql
   {job="docker"}
   ```

2. **Check specific service:**
   ```logql
   {service="crawler"}
   ```

3. **Compare log volumes:**
   ```logql
   # Total logs in last hour
   sum(count_over_time({job="docker"}[1h]))
   ```

4. **Test JSON parsing:**
   ```logql
   {service="crawler"} | json | level="error"
   ```

#### Validate All North Cloud Services

Ensure logs from all services are being collected:
```logql
# Count logs by service in last 5 minutes
sum by (service) (count_over_time({job="docker"}[5m]))
```

Expected services:
- crawler
- source-manager
- classifier
- publisher
- auth
- search
- index-manager
- mcp-north-cloud

### Step 5: Performance Comparison

#### Check Resource Usage
```bash
# Promtail memory usage
docker stats north-cloud-promtail --no-stream

# Alloy memory usage
docker stats north-cloud-alloy --no-stream
```

#### Check Loki Ingestion

In Loki logs:
```bash
docker logs north-cloud-loki | grep "ingester" | tail -10
```

Look for:
- No errors or rate limit warnings
- Smooth ingestion from both collectors

### Step 6: Cutover (When Ready)

When confident Alloy is working correctly (typically after 1 week):

#### Option A: Stop Promtail Service Only (Recommended)
```bash
# Stop Promtail but keep it in config for easy rollback
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop promtail
```

#### Option B: Remove Promtail Completely
```bash
# Edit docker-compose.base.yml and comment out the promtail service
# Then restart
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

#### Verify Cutover
```bash
# Check Promtail is stopped
docker ps | grep promtail  # Should return nothing

# Check Alloy is still running and collecting logs
docker logs north-cloud-alloy | tail -20

# Verify logs still appearing in Grafana
# Query: {job="docker"}
```

## Rollback Procedure

If you encounter issues with Alloy:

### Quick Rollback (Restart Promtail)
```bash
# Restart Promtail
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml start promtail

# Stop Alloy (optional)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop alloy

# Verify Promtail is collecting logs
docker logs north-cloud-promtail | tail -20
```

### Full Rollback (Remove Alloy)
```bash
# Stop and remove Alloy
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop alloy
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml rm -f alloy

# Ensure Promtail is running
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d promtail
```

## Troubleshooting

### Alloy Not Starting

**Check container logs:**
```bash
docker logs north-cloud-alloy
```

**Common issues:**
- **Config syntax error:** Check `/infrastructure/alloy/config.alloy` for HCL syntax errors
- **Docker socket permission:** Ensure Docker socket is mounted read-only
- **Port conflict:** Check if port 12345 is already in use

**Solution:**
```bash
# Validate config manually
docker run --rm -v $(pwd)/infrastructure/alloy/config.alloy:/etc/alloy/config.alloy:ro \
  grafana/alloy:latest fmt --check /etc/alloy/config.alloy
```

### No Logs in Grafana

**Check Alloy is discovering targets:**
1. Open http://localhost:12345
2. Navigate to "Targets" or "Components"
3. Look for `discovery.docker.north_cloud` component
4. Verify Docker containers are listed

**Check Alloy is forwarding to Loki:**
```bash
# Check Alloy logs for write errors
docker logs north-cloud-alloy | grep -i "error\|failed"
```

**Verify Loki connectivity:**
```bash
# Test Loki is reachable from Alloy container
docker exec north-cloud-alloy wget -qO- http://loki:3100/ready
```

### Duplicate Logs in Grafana

**Expected during migration:** Both Promtail and Alloy send logs to Loki simultaneously.

**To differentiate:**
- Promtail logs: Default labels from relabeling
- Alloy logs: Same labels (intentional for compatibility)

**If problematic:**
```bash
# Stop Promtail temporarily to test Alloy alone
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop promtail
```

### Labels Missing or Incorrect

**Check relabeling rules:**
1. Open Alloy UI: http://localhost:12345
2. Navigate to `discovery.relabel.north_cloud` component
3. Verify rules match Promtail configuration

**Common label issues:**
- `service` label empty → Check container name regex
- `project` label missing → Check Docker Compose label filter
- `level` label missing → Check JSON parsing in `loki.process` component

### High Memory Usage

**Check Alloy metrics:**
```bash
# Alloy exposes Prometheus metrics
curl -s http://localhost:12345/metrics | grep alloy_
```

**Compare with Promtail:**
```bash
# Promtail metrics
curl -s http://localhost:9080/metrics | grep promtail_
```

**If Alloy uses significantly more memory:**
- Check for config inefficiencies (excessive relabeling, large buffers)
- Review component pipeline for bottlenecks
- Consult Alloy documentation for tuning parameters

## Configuration File Reference

### Alloy Configuration Location
```
/home/jones/dev/north-cloud/infrastructure/alloy/config.alloy
```

### Key Components

**1. Discovery (discovery.docker)**
- Connects to Docker socket
- Filters containers by label: `com.docker.compose.project=north-cloud`
- Refreshes every 5 seconds

**2. Relabeling (discovery.relabel)**
- Extracts service name from container name or Docker Compose label
- Adds project, container_id, job labels
- Identical logic to Promtail relabel_configs

**3. Source (loki.source.docker)**
- Collects logs from discovered Docker containers
- Forwards to processing pipeline

**4. Processing (loki.process)**
- Stage 1: Parse Docker JSON wrapper (log, stream, time)
- Stage 2: Parse service JSON logs (level, logger, caller, msg, ts, etc.)
- Stage 3: Extract level as label
- Stage 4: Fallback regex for non-JSON logs
- Stage 5-6: Parse timestamps (service logs → Docker wrapper)
- Stage 7: Add stream label (stdout/stderr)
- Stage 8: Preserve JSON structure (output: log field)

**5. Writing (loki.write)**
- Sends logs to Loki at http://loki:3100/loki/api/v1/push
- Timeout: 10s
- Retry: 10 attempts with exponential backoff (500ms to 5m)
- Batch: 100KB or 1 second

## Metrics and Monitoring

### Alloy Metrics Endpoint
```
http://localhost:12345/metrics
```

### Key Metrics to Monitor

**Log Collection:**
- `alloy_loki_source_docker_targets` - Number of active targets
- `alloy_loki_source_docker_entries_total` - Total log entries collected
- `alloy_loki_source_docker_parsing_errors_total` - Parsing failures

**Log Writing:**
- `alloy_loki_write_sent_entries_total` - Entries sent to Loki
- `alloy_loki_write_sent_bytes_total` - Bytes sent to Loki
- `alloy_loki_write_dropped_entries_total` - Dropped entries (errors)

**Performance:**
- `process_resident_memory_bytes` - Alloy memory usage
- `go_goroutines` - Active goroutines

### Grafana Dashboard (Optional)

Create an Alloy monitoring dashboard in Grafana:

1. **Panel 1:** Log ingestion rate
   ```promql
   rate(alloy_loki_source_docker_entries_total[5m])
   ```

2. **Panel 2:** Active targets
   ```promql
   alloy_loki_source_docker_targets
   ```

3. **Panel 3:** Parsing errors
   ```promql
   rate(alloy_loki_source_docker_parsing_errors_total[5m])
   ```

4. **Panel 4:** Memory usage
   ```promql
   process_resident_memory_bytes{job="alloy"}
   ```

## Next Steps After Migration

### 1. Remove Promtail (After Successful Cutover)

**Edit docker-compose.base.yml:**
- Comment out or remove the `promtail` service definition
- Remove `promtail_positions` volume (optional, keep for rollback)

**Clean up volumes:**
```bash
# Remove Promtail position file (optional)
docker volume rm north-cloud_promtail_positions
```

### 2. Update Documentation

**Files to update:**
- `/README.md` - Mention Alloy instead of Promtail
- `/infrastructure/grafana/README.md` - Update log collector references
- `/CLAUDE.md` - Replace Promtail architecture with Alloy
- Any team wiki or operational runbooks

### 3. Archive Promtail Configuration

**Create archive directory:**
```bash
mkdir -p /home/jones/dev/north-cloud/infrastructure/_archived/promtail
mv /home/jones/dev/north-cloud/infrastructure/promtail/* \
   /home/jones/dev/north-cloud/infrastructure/_archived/promtail/
```

**Keep for reference:**
- Rollback capability (if needed within 1-2 months)
- Historical documentation
- Configuration comparison for troubleshooting

### 4. Monitor Long-Term Performance

**Set calendar reminders:**
- 1 week: Quick performance check
- 1 month: Comprehensive review (memory, CPU, log volumes)
- 3 months: Finalize migration (remove Promtail permanently)

**Key questions:**
- Are all services' logs being captured?
- Is Alloy using acceptable resources?
- Are there any parsing errors or dropped logs?
- Do Grafana queries perform as expected?

## Resources

### Official Documentation
- [Grafana Alloy Documentation](https://grafana.com/docs/alloy/latest/)
- [Run Grafana Alloy in Docker](https://grafana.com/docs/alloy/latest/set-up/install/docker/)
- [Monitor Docker containers with Alloy](https://grafana.com/docs/alloy/latest/monitor/monitor-docker-containers/)
- [Migrate from Promtail to Alloy](https://grafana.com/docs/alloy/latest/set-up/migrate/from-promtail/)
- [loki.source.docker Component Reference](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.docker/)

### Community Resources
- [Promtail EOL Announcement (Grafana Community)](https://community.grafana.com/t/promtail-end-of-life-eol-march-2026-how-to-migrate-to-grafana-alloy-for-existing-loki-server-deployments/159636)
- Grafana Slack: #grafana-alloy channel
- Grafana Community Forums: https://community.grafana.com

### North Cloud Specific
- Configuration: `/infrastructure/alloy/config.alloy`
- Migration documentation: `/infrastructure/alloy/MIGRATION.md` (this file)
- Original Promtail config: `/infrastructure/promtail/promtail-config.yml`
- Architecture documentation: `/CLAUDE.md`

## Support and Questions

**For issues with this migration:**
1. Check Alloy logs: `docker logs north-cloud-alloy`
2. Review Alloy UI: http://localhost:12345
3. Consult official Grafana Alloy documentation
4. Search Grafana Community Forums
5. Open an issue in the North Cloud repository

**Emergency rollback:** See [Rollback Procedure](#rollback-procedure) above

---

**Migration Prepared:** January 10, 2026
**Promtail EOL Date:** March 2, 2026
**Recommended Completion:** February 2026 (before EOL)

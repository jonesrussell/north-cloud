# Grafana Alloy Configuration

## Overview

Grafana Alloy is the next-generation telemetry collector for the North Cloud logging infrastructure. It replaces Promtail, which reaches End-of-Life (EOL) on March 2, 2026.

## Quick Start

### Start Alloy

```bash
# Ensure ALLOY_PORT is set in .env
echo "ALLOY_PORT=12345" >> .env

# Start services
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Verify Alloy is running
docker logs -f north-cloud-alloy
```

### Access Alloy UI

Open http://localhost:12345 in your browser to access Alloy's debugging UI.

**Features:**
- Component graph showing data flow
- Target discovery (Docker containers)
- Relabeling rules visualization
- Log collection statistics
- Real-time metrics

### Test Log Collection

```bash
# Generate some logs
docker logs north-cloud-crawler | tail -20

# Verify Alloy collected them
docker logs north-cloud-alloy | grep "loki.write" | tail -5

# Query in Grafana
# Open http://localhost:3000 → Explore → Loki
# Query: {service="crawler"}
```

## Files

- **`config.alloy`** - Main Alloy configuration (HCL format)
- **`MIGRATION.md`** - Complete migration guide from Promtail
- **`IMPLEMENTATION_SUMMARY.md`** - Implementation details and testing checklist
- **`README.md`** - This file (quick reference)

## Configuration

### Format: HCL (HashiCorp Configuration Language)

Alloy uses a component-based architecture with explicit data flow:

```
discovery.docker → discovery.relabel → loki.source.docker → loki.process → loki.write
```

### Key Components

1. **`discovery.docker`** - Discovers Docker containers
2. **`discovery.relabel`** - Extracts labels from container metadata
3. **`loki.source.docker`** - Collects logs from containers
4. **`loki.process`** - Parses and transforms logs
5. **`loki.write`** - Forwards logs to Loki

### Log Processing Pipeline

1. Parse Docker JSON wrapper (log, stream, time)
2. Parse service JSON logs (level, logger, caller, msg, etc.)
3. Extract level as label for filtering
4. Fallback regex for non-JSON logs
5. Parse timestamps (service logs → Docker wrapper)
6. Add stream label (stdout/stderr)
7. Preserve JSON structure for Grafana re-parsing

## Why Alloy?

### Promtail EOL Timeline
- **February 13, 2025:** Promtail enters Long-Term Support (LTS)
- **February 28, 2026:** Commercial support ends
- **March 2, 2026:** Promtail End-of-Life (no security patches)

### Alloy Advantages
- ✅ Active development and long-term support
- ✅ Vendor-neutral (supports Loki, Tempo, Prometheus, etc.)
- ✅ Unified collector for logs, metrics, and traces
- ✅ Better performance and resource usage
- ✅ Modern component-based architecture
- ✅ Built-in debugging UI

## Common Commands

### View Alloy Logs
```bash
docker logs north-cloud-alloy
docker logs -f north-cloud-alloy  # Follow mode
```

### Check Alloy Status
```bash
docker ps | grep alloy
docker stats north-cloud-alloy --no-stream
```

### Restart Alloy
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml restart alloy
```

### Stop Alloy
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop alloy
```

### Validate Configuration
```bash
docker run --rm -v $(pwd)/config.alloy:/etc/alloy/config.alloy:ro \
  grafana/alloy:latest fmt --check /etc/alloy/config.alloy
```

## Troubleshooting

### Alloy Not Starting

**Check logs:**
```bash
docker logs north-cloud-alloy
```

**Common issues:**
- Config syntax error → Check `config.alloy` for HCL syntax
- Docker socket permission → Verify socket mounted read-only
- Port conflict → Check if port 12345 is in use

### No Logs in Grafana

**Check discovery:**
1. Open http://localhost:12345
2. Navigate to "Targets" or "Components"
3. Verify Docker containers are listed

**Check forwarding:**
```bash
docker logs north-cloud-alloy | grep -i "error\|failed"
```

**Test Loki connectivity:**
```bash
docker exec north-cloud-alloy wget -qO- http://loki:3100/ready
```

### Labels Missing

**Check relabeling:**
1. Open Alloy UI: http://localhost:12345
2. Check `discovery.relabel.north_cloud` component
3. Verify rules match expected labels

## Metrics

### Alloy Metrics Endpoint
```
http://localhost:12345/metrics
```

### Key Metrics
- `alloy_loki_source_docker_targets` - Active targets
- `alloy_loki_source_docker_entries_total` - Entries collected
- `alloy_loki_write_sent_entries_total` - Entries sent to Loki
- `alloy_loki_write_dropped_entries_total` - Dropped entries
- `process_resident_memory_bytes` - Memory usage

## Documentation

### Internal
- [Migration Guide](MIGRATION.md) - Complete Promtail → Alloy migration
- [Implementation Summary](IMPLEMENTATION_SUMMARY.md) - Technical details
- [Architecture Documentation](../../CLAUDE.md) - North Cloud architecture

### External
- [Grafana Alloy Docs](https://grafana.com/docs/alloy/latest/)
- [Run in Docker](https://grafana.com/docs/alloy/latest/set-up/install/docker/)
- [Monitor Docker Containers](https://grafana.com/docs/alloy/latest/monitor/monitor-docker-containers/)
- [Migrate from Promtail](https://grafana.com/docs/alloy/latest/set-up/migrate/from-promtail/)
- [loki.source.docker](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.docker/)

## Support

For issues or questions:
1. Check [MIGRATION.md](MIGRATION.md) troubleshooting section
2. Review Alloy logs: `docker logs north-cloud-alloy`
3. Check Alloy UI: http://localhost:12345
4. Consult [official docs](https://grafana.com/docs/alloy/latest/)
5. Ask in [Grafana Community](https://community.grafana.com)

---

**Status:** ✅ Ready for testing
**Promtail EOL:** March 2, 2026
**Migration Deadline:** February 2026

# Grafana Centralized Logging

This directory contains the configuration for Grafana, which provides a web-based UI for viewing and analyzing logs collected by Loki from all North Cloud services.

## Overview

The North Cloud logging stack consists of three components:

1. **Grafana Loki** - Log aggregation and storage
2. **Grafana Alloy** - Log collection from Docker containers
3. **Grafana** - Web UI for querying and visualizing logs

```
Services (JSON logs) → Docker → Alloy → Loki → Grafana (Web UI)
```

## Architecture

### Log Flow

1. All North Cloud services write structured JSON logs to stdout using Zap logger
2. Docker captures these logs using the json-file driver
3. Grafana Alloy discovers and collects logs from Docker containers via the Docker socket
4. Alloy parses JSON logs and extracts labels (service, level, container)
5. Alloy processes logs and forwards them to Loki
6. Loki stores logs with label-based indexing
7. Grafana provides a web interface to query and visualize logs from Loki

### Components

#### Loki
- **Image**: `grafana/loki:2.9.10`
- **Port**: 3100 (HTTP API)
- **Storage**: Filesystem-based (configurable to S3/MinIO)
- **Retention**: 7 days (dev) / 30 days (prod)
- **Configuration**: `/infrastructure/loki/loki-config*.yml`

#### Grafana Alloy
- **Image**: `grafana/alloy:latest`
- **Port**: 12345 (debugging UI), metrics on same port
- **Collects**: Logs from Docker containers via Docker socket
- **Configuration**: `/infrastructure/alloy/config.alloy` (HCL format)
- **Docs**: `/infrastructure/alloy/README.md`

#### Grafana
- **Image**: `grafana/grafana:10.4.8`
- **Port**: 3000 (Web UI)
- **Datasources**: Auto-provisioned Loki and Elasticsearch datasources
- **Dashboards**: North Cloud Logs, North Cloud → StreetCode Pipeline (Loki + ES)
- **Configuration**: `/infrastructure/grafana/provisioning/`

## Quick Start

### Starting the Logging Stack

```bash
# Development environment
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d loki alloy grafana

# Wait for services to be healthy
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps loki alloy grafana
```

### Accessing Grafana

1. Open your browser to [http://localhost:3000](http://localhost:3000)
2. **Development**: Anonymous access enabled (no login required)
3. **Production**: Login with credentials from `.env`:
   - Username: `${GRAFANA_ADMIN_USER}` (default: admin)
   - Password: `${GRAFANA_ADMIN_PASSWORD}` (default: changeme)

### Viewing Logs

#### Option 1: Pre-configured Dashboard

1. Navigate to **Dashboards** → **North Cloud** folder
2. Open **North Cloud Logs** (per-service log volume and stream) or **North Cloud → StreetCode Pipeline** (classifier ES + publisher/StreetCode Loki)
3. On North Cloud Logs, use filters:
   - **Service**: Select one or more services (crawler, publisher, etc.)
   - **Level**: Filter by log level (debug, info, warn, error)
   - **Search**: Free-text search across log messages

#### Option 2: Explore (Ad-hoc Queries)

1. Navigate to **Explore** (compass icon in left sidebar)
2. Ensure **Loki** is selected as the datasource
3. Write LogQL queries:

```logql
# All logs from crawler service
{service="crawler"}

# All errors across all services
{project="north-cloud", level="error"}

# Logs containing specific text
{service="publisher"} |= "published article"

# Logs NOT containing text
{service="crawler"} != "health check"

# Combine filters
{service="classifier", level=~"error|warn"} |= "failed"
```

## LogQL Query Language

LogQL is Loki's query language, similar to Prometheus' PromQL.

### Label Selectors

```logql
# Exact match
{service="crawler"}

# Regex match
{service=~"crawler|publisher"}

# Not equal
{service!="nginx"}

# Multiple labels
{service="crawler", level="error"}
```

### Log Stream Filters

```logql
# Contains text (case-sensitive)
{service="crawler"} |= "error"

# Does not contain
{service="crawler"} != "health"

# Regex match
{service="crawler"} |~ "error|warning"

# Regex does not match
{service="crawler"} !~ "debug.*info"
```

### Parsing and Filtering

```logql
# Parse JSON logs
{service="crawler"} | json

# Extract specific field
{service="crawler"} | json | line_format "{{.msg}}"

# Filter by parsed field
{service="crawler"} | json | status_code >= 400
```

### Aggregations

```logql
# Count logs per service (last 5 minutes)
sum by (service) (count_over_time({project="north-cloud"}[5m]))

# Count errors per service
sum by (service) (count_over_time({level="error"}[1h]))

# Rate of logs per second
rate({service="crawler"}[5m])
```

## Common Query Examples

### Find All Errors

```logql
{project="north-cloud", level="error"}
```

### Find Slow Requests (>1s duration)

```logql
{service=~"crawler|publisher"} | json | duration > 1000
```

### Find HTTP 5xx Errors

```logql
{service="nginx"} | json | status_code >= 500
```

### View Crawler Job Executions

```logql
{service="crawler"} |= "job execution" | json
```

### Count Logs by Service (Last Hour)

```logql
sum by (service) (count_over_time({project="north-cloud"}[1h]))
```

### Alert on Error Rate

```logql
sum by (service) (
  rate({level="error"}[5m])
) > 0.1  # More than 0.1 errors per second
```

## Configuration

### Environment Variables

Defined in `.env.example`:

```bash
# Loki
LOKI_PORT=3100
LOKI_RETENTION_DAYS=7              # Dev: 7 days, Prod: 30 days
LOKI_INGESTION_RATE_MB=5
LOKI_INGESTION_BURST_MB=10

# Grafana Alloy
ALLOY_PORT=12345

# Grafana
GRAFANA_PORT=3000
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=changeme    # CHANGE IN PRODUCTION
GRAFANA_ANONYMOUS_ENABLED=false
GRAFANA_ROOT_URL=http://localhost:3000
GRAFANA_LOG_MODE=console
GRAFANA_LOG_LEVEL=info
```

### Loki Configuration Files

- **Base**: `/infrastructure/loki/loki-config.yml` (not used directly)
- **Development**: `/infrastructure/loki/loki-config.dev.yml`
  - 7-day retention
  - Debug logging
  - Higher ingestion limits
  - More frequent compaction
- **Production**: `/infrastructure/loki/loki-config.prod.yml`
  - 30-day retention
  - Info logging
  - Conservative limits
  - Optimized for stability

### Grafana Alloy Configuration

File: `/infrastructure/alloy/config.alloy` (HCL format)

**Components**:
1. **discovery.docker** - Discovers all North Cloud Docker containers
2. **discovery.relabel** - Extracts labels from container metadata
3. **loki.source.docker** - Collects logs from discovered containers
4. **loki.process** - Parses and transforms logs (JSON, logfmt, regex)
5. **loki.write** - Forwards processed logs to Loki

**Labels Extracted**:
- `service` - Service name from Docker Compose (e.g., `crawler`)
- `project` - Project name (`north-cloud`)
- `container_id` - Docker container ID (first 12 chars)
- `job` - Fixed to `docker`
- `level` - Log level extracted from service logs (debug, info, warn, error)
- `stream` - stdout or stderr

**Configuration Details**: See `/infrastructure/alloy/README.md` for comprehensive documentation

### Grafana Provisioning

#### Datasources

File: `/infrastructure/grafana/provisioning/datasources/loki.yml`

Auto-configures Loki datasource on startup:
- URL: `http://loki:3100`
- Default datasource: Yes
- Max lines: 1000

#### Dashboards

Provider: `/infrastructure/grafana/provisioning/dashboards/dashboards.yml`  
All `.json` files in `dashboards/` are loaded into the **North Cloud** folder.

- **north-cloud-logs.json** – Pre-configured dashboard includes:
- **Log Volume by Service** - Bar chart of logs per service
- **Logs by Level** - Time series of log levels
- **Logs** - Live log stream with filtering
- **Error Count by Service** - Bar gauge of recent errors
- **Recent Errors** - Table of last 10 errors

- **north-cloud-pipeline.json** – North Cloud → StreetCode Pipeline (Loki + ES): classifier ES panels, publisher/StreetCode Loki panels.

**If a provisioned dashboard does not appear:** go to **Dashboards** → **Browse** → open the **North Cloud** folder (not General). On the server, ensure the JSON file exists and restart Grafana after deploy (see [Provisioned dashboard not visible](#provisioned-dashboard-not-visible)).

## Operational Tasks

### Viewing Logs Without Grafana

```bash
# Query Loki API directly
curl -G 'http://localhost:3100/loki/api/v1/query' \
  --data-urlencode 'query={service="crawler"}' \
  | jq '.data.result'

# Stream logs in real-time
curl -G 'http://localhost:3100/loki/api/v1/tail' \
  --data-urlencode 'query={service="crawler"}' \
  --data-urlencode 'limit=10'
```

### Checking Service Health

```bash
# Loki
curl http://localhost:3100/ready

# Grafana Alloy
curl http://localhost:12345/ready

# Grafana
curl http://localhost:3000/api/health
```

### Viewing Service Metrics

```bash
# Loki metrics (Prometheus format)
curl http://localhost:3100/metrics

# Grafana Alloy metrics
curl http://localhost:12345/metrics
```

### Managing Retention

Loki automatically deletes logs older than the configured retention period:

- **Development**: 7 days (168 hours)
- **Production**: 30 days (720 hours)

To manually trigger compaction:

```bash
# Not typically needed - Loki handles this automatically
docker exec north-cloud-loki wget -qO- http://localhost:3100/loki/api/v1/delete
```

### Backup and Restore

#### Backup Loki Data

```bash
# Stop Loki
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop loki

# Backup volume
docker run --rm \
  -v north-cloud_loki_data:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/loki-backup-$(date +%Y%m%d).tar.gz -C /data .

# Start Loki
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml start loki
```

#### Restore Loki Data

```bash
# Stop Loki
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml stop loki

# Restore volume
docker run --rm \
  -v north-cloud_loki_data:/data \
  -v $(pwd)/backups:/backup \
  alpine sh -c "cd /data && tar xzf /backup/loki-backup-20260108.tar.gz"

# Start Loki
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml start loki
```

#### Export Grafana Dashboards

```bash
# Get dashboard UID from Grafana UI or:
curl -s http://admin:changeme@localhost:3000/api/search | jq '.[] | {title, uid}'

# Export dashboard
curl -s http://admin:changeme@localhost:3000/api/dashboards/uid/north-cloud-logs \
  | jq '.dashboard' > dashboard-backup.json
```

## Troubleshooting

### Logs Not Appearing in Grafana

1. **Check Alloy is running and healthy**:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps alloy
   curl http://localhost:12345/ready
   ```

2. **Check Alloy logs**:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs alloy
   ```

3. **Verify Docker socket is mounted**:
   ```bash
   docker inspect north-cloud-alloy | jq '.[].Mounts'
   ```

4. **Check Alloy UI for target discovery**:
   - Open http://localhost:12345 in browser
   - Navigate to "Targets" or "Components"
   - Verify Docker containers are listed

4. **Check Loki is receiving logs**:
   ```bash
   curl http://localhost:3100/metrics | grep loki_ingester_streams_created_total
   ```

5. **Verify service logs are JSON formatted**:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs crawler | head -5
   ```

### Provisioned dashboard not visible

1. **Check the folder**: Provisioned dashboards live in **North Cloud**, not General. In Grafana: **Dashboards** → **Browse** → open **North Cloud**.
2. **Verify the file on the server** (e.g. production at `/opt/north-cloud`):
   ```bash
   ls -la /opt/north-cloud/infrastructure/grafana/provisioning/dashboards/
   ```
   You should see `north-cloud-pipeline.json` and `north-cloud-logs.json`. If `north-cloud-pipeline.json` is missing, pull/deploy the repo so the file is present.
   **Tip:** If Cursor has the **North Cloud (Production)** MCP server configured (`.cursor/mcp.json`), you can run production checks (e.g. `list_indexes`, `search_articles`) via MCP instead of SSH + docker exec.
3. **Restart Grafana** after deploying new or updated dashboard JSON:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart grafana
   ```
4. **Check Grafana logs** for provisioning errors:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs grafana 2>&1 | grep -i provision
   ```

### Elasticsearch: "No date field named @timestamp found"

Classified content indexes use **`crawled_at`** as the time field, not `@timestamp`. When saving the Elasticsearch datasource (e.g. after setting Index name to `*_classified_content`):

1. In **Connections** → **Data sources** → **Elasticsearch**, open the datasource.
2. Under **Elasticsearch details**, set **Time field name** to **`crawled_at`** (not @timestamp).
3. Save & test.

Provisioning already sets `timeField: crawled_at` in `provisioning/datasources/elasticsearch.yml`; restart Grafana so the provisioned config is loaded, or set it manually as above.

### Elasticsearch panels show "No data"

The Connection tab only shows URL; the index and time field are set **further down** the same page:

1. Open **Connections** → **Data sources** → **elasticsearch**.
2. Scroll to the **Elasticsearch details** section (below Connection and Authentication).
3. Set **Index name** to **`*_classified_content`**. (Without this, Grafana does not query your classified indexes.)
4. Set **Time field name** to **`crawled_at`**.
5. Click **Save & test**.

If you use provisioning, ensure `provisioning/datasources/elasticsearch.yml` has `database: '*_classified_content'` and `jsonData.timeField: crawled_at`, then restart Grafana. Set the dashboard time range to **Last 7 days** (or **Last 30 days**) so the time filter includes existing data.

**Why `content_type.keyword`:** Some production indices map `content_type` as text (aggregations disabled). The pipeline dashboard uses `content_type.keyword` for the Content Type and Crime Relevance panels so all shards succeed; indices with `content_type` as keyword still work via the `.keyword` subfield where present.

### Grafana Shows "Loki: Bad Gateway"

1. **Check Loki is running**:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps loki
   ```

2. **Verify network connectivity**:
   ```bash
   docker exec north-cloud-grafana wget -qO- http://loki:3100/ready
   ```

3. **Check datasource configuration**:
   - Grafana UI → Configuration → Data sources → Loki
   - Click "Test" button

### High Memory Usage

1. **Check Loki memory**:
   ```bash
   docker stats north-cloud-loki
   ```

2. **Reduce ingestion rate** in `.env`:
   ```bash
   LOKI_INGESTION_RATE_MB=3
   LOKI_INGESTION_BURST_MB=6
   ```

3. **Reduce retention period**:
   - Edit `/infrastructure/loki/loki-config.dev.yml`
   - Change `retention_period: 168h` to `retention_period: 72h` (3 days)

4. **Enable log sampling** in Alloy (advanced):
   - Edit `/infrastructure/alloy/config.alloy`
   - Add `drop` stage in `loki.process` component to filter logs

### Logs Missing from Specific Service

1. **Check service is running**:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps
   ```

2. **Verify service has correct labels**:
   ```bash
   docker inspect north-cloud-crawler | jq '.[].Config.Labels'
   ```

3. **Check Alloy targets**:
   - Open http://localhost:12345 in browser
   - Navigate to "Targets" or "Components"
   - Verify service appears in discovered containers list

4. **Query Loki for service**:
   ```bash
   curl -G 'http://localhost:3100/loki/api/v1/label/service/values'
   ```

## Performance Tuning

### For High-Volume Logging

If you have >100 log lines per second:

1. **Adjust Alloy batch settings** in `/infrastructure/alloy/config.alloy`:
   ```hcl
   loki.write "north_cloud" {
     endpoint {
       batch_wait = "2s"
       batch_size = "512KiB"
     }
   }
   ```

2. **Increase Loki limits**:
   ```bash
   LOKI_INGESTION_RATE_MB=10
   LOKI_INGESTION_BURST_MB=20
   ```

3. **Consider using MinIO/S3 for storage**:
   - Edit `/infrastructure/loki/loki-config.prod.yml`
   - Change `storage_config` to use `s3` instead of `filesystem`

### For Low-Resource Environments

If you have limited RAM/CPU:

1. **Reduce retention**:
   ```bash
   LOKI_RETENTION_DAYS=3
   ```

2. **Limit concurrent queries** in Loki config:
   ```yaml
   querier:
     max_concurrent: 5  # Default: 10
   ```

3. **Sample logs** in Alloy (drop debug logs):
   - Edit `/infrastructure/alloy/config.alloy`
   - Add a `drop` stage in `loki.process` component to filter debug logs

## Security Considerations

### Production Deployment

1. **Change default Grafana password**:
   ```bash
   GRAFANA_ADMIN_PASSWORD="$(openssl rand -base64 32)"
   ```

2. **Disable anonymous access**:
   ```bash
   GRAFANA_ANONYMOUS_ENABLED=false
   ```

3. **Enable authentication on Loki** (optional):
   - Edit `/infrastructure/loki/loki-config.prod.yml`
   - Set `auth_enabled: true`
   - Configure tenants

4. **Restrict network access**:
   - Use firewall rules to limit access to ports 3000, 3100, 12345
   - Only expose Grafana (3000) externally

5. **Use HTTPS**:
   - Configure nginx reverse proxy for Grafana
   - Use Let's Encrypt certificates

### Log Data Privacy

Logs may contain sensitive information. Consider:

1. **Scrubbing sensitive data** in application logs
2. **Encrypting log data at rest** (filesystem encryption)
3. **Access control** for Grafana dashboards
4. **Audit logging** of Grafana user actions

## Integration with Other Tools

### Linking to Pyroscope Profiles

Grafana can link from logs to Pyroscope profiling data:

1. Add `trace_id` field to logs
2. Configure derived fields in Loki datasource
3. Click log entry → Jump to profile

### Alerting

Grafana can send alerts based on log patterns:

1. Navigate to **Alerting** → **Alert rules**
2. Create new rule:
   ```logql
   sum by (service) (
     rate({level="error"}[5m])
   ) > 0.5
   ```
3. Configure notification channels (email, Slack, PagerDuty)

## Additional Resources

- [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/)
- [LogQL Query Language](https://grafana.com/docs/loki/latest/logql/)
- [Grafana Alloy Documentation](https://grafana.com/docs/alloy/latest/)
- [Alloy Docker Monitoring](https://grafana.com/docs/alloy/latest/monitor/monitor-docker-containers/)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
- [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/)

## Support

For issues with the logging stack:

1. Check this README for troubleshooting steps
2. Review service logs: `docker compose logs loki alloy grafana`
3. Check Alloy UI: http://localhost:12345 (debugging interface)
4. Consult Alloy documentation: `/infrastructure/alloy/README.md`
5. Check Grafana community forums: https://community.grafana.com/
6. Report bugs in the North Cloud repository

# Grafana Dashboard Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace 2 existing Grafana dashboards with 3 enterprise-grade dashboards + provisioned alerting rules.

**Architecture:** Grafana provisioned dashboards (JSON) and alerting rules (YAML) deployed to `infrastructure/grafana/provisioning/`. Dashboards use Loki (uid: `loki`) and Elasticsearch (uid: `elasticsearch`, index: `*_classified_content`, time field: `crawled_at`) datasources.

**Tech Stack:** Grafana 10.4.8, Loki LogQL, Elasticsearch aggregations, Grafana provisioning YAML

**Design spec:** `docs/plans/2026-02-14-grafana-dashboard-redesign.md` — contains every panel, query, visualization type, threshold, and grid position.

---

### Task 1: Create Pipeline Operations Dashboard

**Files:**
- Create: `infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline-ops.json`

**Step 1:** Generate the complete dashboard JSON following the design spec "Dashboard 1: Pipeline Operations" section. The dashboard has:
- UID: `north-cloud-pipeline-ops`, title: "Pipeline Operations"
- Default range: 24h, refresh: 30s, tags: `north-cloud`, `pipeline`
- Dashboard links bar (Pipeline Ops, Deployer Sites, Service Logs, Explore Loki)
- Row 1: Pipeline Throughput — 5 stat panels (Sources Discovered, Articles Classified, Batches Published, Articles Published, Pipeline Errors)
- Row 2: Pipeline Flow — 2 timeseries panels (Throughput by Stage stacked bars, Error Rate by Service lines)
- Row 3: Content Analytics — 4 panels from Elasticsearch (Content Type piechart, Quality Score histogram, Crime Relevance piechart, Mining Relevance piechart)
- Row 4: Publisher Routing — 2 panels (Articles per Redis Channel barchart, Classifier Throughput Rate timeseries)
- Row 5: Suspected Misclassifications — collapsed row, 1 table panel from ES
- Row 6: Recent Pipeline Errors — collapsed row, 1 logs panel

Use existing dashboard JSON patterns (from `north-cloud-pipeline.json`) for structure. All exact queries, thresholds, and display options are in the design spec.

**Step 2:** Verify JSON is valid: `python3 -c "import json; json.load(open('infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline-ops.json'))"`

---

### Task 2: Create Deployer Sites Dashboard

**Files:**
- Create: `infrastructure/grafana/provisioning/dashboards/north-cloud-deployer-sites.json`

**Step 1:** Generate the complete dashboard JSON following the design spec "Dashboard 2: Deployer Sites" section. The dashboard has:
- UID: `north-cloud-deployer-sites`, title: "Deployer Sites"
- Default range: 6h, refresh: 30s, tags: `north-cloud`, `deployer`, `laravel`
- Variables: `site` (custom multi-select: streetcode, orewire, coforge, movies-of-war), `level` (custom multi-select)
- Dashboard links bar
- Row 1: Site Health Overview — 4 columns of stat panels (Log Volume + Errors per site = 8 stat panels)
- Row 2: Error Trends — 1 full-width timeseries (Error Rate by Site)
- Row 3: Ingestion Metrics — 6 stat panels (Ingested + Skipped per consumer site)
- Rows 4-7: Per-Site Log Streams — 4 collapsed rows, each with 1 logs panel filtered by site and `$level`

**Step 2:** Verify JSON is valid.

---

### Task 3: Create Service Logs Dashboard

**Files:**
- Create: `infrastructure/grafana/provisioning/dashboards/north-cloud-service-logs.json`

**Step 1:** Generate the complete dashboard JSON following the design spec "Dashboard 3: Service Logs" section. The dashboard has:
- UID: `north-cloud-service-logs`, title: "Service Logs"
- Default range: 1h, refresh: 10s, tags: `north-cloud`, `logs`
- Variables: `service` (Loki label_values query, multi-select), `level` (custom multi-select), `search` (textbox)
- Dashboard links bar
- Row 1: Log Volume — 2 timeseries (by Service w=16, by Level w=8)
- Row 2: Error Summary — bargauge (Errors by Service Last 5m) + table (Top Error Messages)
- Row 3: Log Stream — full-width logs panel with JSON parsing and line_format

**Step 2:** Verify JSON is valid.

---

### Task 4: Create Alerting Rules

**Files:**
- Create: `infrastructure/grafana/provisioning/alerting/alerts.yml`

**Step 1:** Create the alerting provisioning YAML with 4 alert rules from the design spec "Alerting Rules" section:
1. Pipeline stall — no classifications in 2h
2. Error spike — >10 errors/min for 5 min
3. Publisher zero output — no articles published in 4h
4. Deployer site silence — no logs from streetcode/orewire/coforge in 30 min

Use Grafana's provisioning alerting format (apiVersion 1, groups with rules).

**Step 2:** Verify YAML is valid: `python3 -c "import yaml; yaml.safe_load(open('infrastructure/grafana/provisioning/alerting/alerts.yml'))"`

---

### Task 5: Remove Old Dashboards

**Files:**
- Delete: `infrastructure/grafana/provisioning/dashboards/north-cloud-logs.json`
- Delete: `infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline.json`

**Step 1:** Delete the old dashboard files. Grafana auto-removes them (`disableDeletion: false`).

---

### Task 6: Deploy to Production and Verify

**Step 1:** Copy new files to production:
```bash
scp infrastructure/grafana/provisioning/dashboards/north-cloud-*.json jones@northcloud.biz:/opt/north-cloud/infrastructure/grafana/provisioning/dashboards/
scp -r infrastructure/grafana/provisioning/alerting jones@northcloud.biz:/opt/north-cloud/infrastructure/grafana/provisioning/
```

**Step 2:** Remove old dashboards on production:
```bash
ssh jones@northcloud.biz "rm -f /opt/north-cloud/infrastructure/grafana/provisioning/dashboards/north-cloud-logs.json /opt/north-cloud/infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline.json"
```

**Step 3:** Restart Grafana to pick up new provisioning:
```bash
ssh jones@northcloud.biz "cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart grafana"
```

**Step 4:** Verify dashboards load — check Grafana API:
```bash
ssh jones@northcloud.biz "docker exec north-cloud-grafana-1 wget -qO- 'http://localhost:3000/api/search?type=dash-db' 2>&1"
```

**Step 5:** Commit all changes.

# Grafana Dashboard Redesign — Enterprise-Grade Observability

**Date**: 2026-02-14
**Scope**: Replace 2 existing dashboards with 3 purpose-built dashboards + alerting rules

## Architecture

```
Dashboard 1: Pipeline Operations  (command center — full pipeline health)
Dashboard 2: Deployer Sites       (consumer health — 4 Laravel apps)
Dashboard 3: Service Logs          (deep-dive log explorer)
Alert Rules: Provisioned YAML      (pipeline stall, error spike, site silence)
```

**Datasources**: Loki (uid: `loki`), Elasticsearch (uid: `elasticsearch`)
**Grafana version**: 10.4.8
**ES index pattern**: `*_classified_content` (time field: `crawled_at`)

---

## Verified Log Messages (from production Loki)

These are the exact `msg` values to match in LogQL queries:

| Service | msg | Key JSON fields |
|---------|-----|-----------------|
| publisher | `Published article to channel` | `article_id`, `title`, `channel` |
| publisher | `Batch complete` | `articles_in_batch`, `articles_published_total` |
| publisher | `Processing articles batch` | `batch_size`, `articles_fetched_total` |
| publisher | `Discovered classified content indexes` | `count` |
| classifier | `[Processor] Classification complete` | `content_id`, `content_type`, `quality_score`, `topics` |
| classifier | `[Processor] Batch processing complete` | `total`, `success`, `errors`, `duration_ms`, `items_per_second` |
| classifier | `[Processor] Successfully indexed classified content` | `count` |
| crawler | `Archived job logs` | `job_id`, `execution_id` |
| streetcode | `Article processed` | (Laravel context array) |
| streetcode | `Skipping non-core-crime` | `crime_relevance`, `title` |

All Go services log as structured JSON with fields: `level`, `ts`, `caller`, `msg`, `service`.
Laravel sites log as: `[timestamp] environment.LEVEL: message {"context"}`.

---

## Dashboard 1: Pipeline Operations

**UID**: `north-cloud-pipeline-ops`
**Title**: Pipeline Operations
**Default time range**: Last 24 hours
**Refresh**: 30s
**Tags**: `north-cloud`, `pipeline`

### Variables

None — this is a pipeline-wide view. All panels are pre-configured.

---

### Row 1: Pipeline Throughput

Collapsed: no. Five stat panels showing the pipeline stages left-to-right.

#### Panel 1.1: Sources Discovered

- **Type**: stat
- **Width**: 4 (of 24)
- **Datasource**: Loki
- **Description**: Number of ES classified_content indexes the publisher found
- **Query (instant)**:
  ```logql
  max_over_time(
    {service="publisher"} |= "Discovered classified content indexes"
      | json count="count"
      | unwrap count [$__range]
  )
  ```
- **Unit**: short
- **Color mode**: background, green
- **Graph mode**: none

#### Panel 1.2: Articles Classified

- **Type**: stat
- **Width**: 5
- **Datasource**: Loki
- **Description**: Individual articles classified
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="classifier"} |= "[Processor] Classification complete" [$__range]
  )
  ```
- **Unit**: short
- **Color mode**: background, blue
- **Graph mode**: area (sparkline)

#### Panel 1.3: Batches Published

- **Type**: stat
- **Width**: 5
- **Datasource**: Loki
- **Description**: Publisher batch cycles completed
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="publisher"} |= "Batch complete" [$__range]
  )
  ```
- **Unit**: short
- **Color mode**: background, blue
- **Graph mode**: area

#### Panel 1.4: Articles Published

- **Type**: stat
- **Width**: 5
- **Datasource**: Loki
- **Description**: Individual articles pushed to Redis channels
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="publisher"} |= "Published article to channel" [$__range]
  )
  ```
- **Unit**: short
- **Color mode**: background, green
- **Graph mode**: area

#### Panel 1.5: Pipeline Errors

- **Type**: stat
- **Width**: 5
- **Datasource**: Loki
- **Description**: Errors across all pipeline services
- **Query (instant)**:
  ```logql
  sum(count_over_time(
    {service=~"crawler|classifier|publisher", level="error"} [$__range]
  ))
  ```
- **Unit**: short
- **Color mode**: background
- **Thresholds**: 0 = green, 1 = yellow, 10 = red
- **Graph mode**: area

---

### Row 2: Pipeline Flow Over Time

Collapsed: no. Two time series panels side by side.

#### Panel 2.1: Throughput by Stage

- **Type**: timeseries
- **Width**: 14
- **Datasource**: Loki
- **Description**: Articles flowing through each pipeline stage over time
- **Queries** (3 queries, stacked):

  **A — Classified**:
  ```logql
  sum(count_over_time(
    {service="classifier"} |= "[Processor] Classification complete" [$__interval]
  ))
  ```
  Legend: `Classified`

  **B — Published**:
  ```logql
  sum(count_over_time(
    {service="publisher"} |= "Published article to channel" [$__interval]
  ))
  ```
  Legend: `Published`

  **C — Consumed**:
  ```logql
  sum(count_over_time(
    {service=~"streetcode|orewire|coforge"} |= "Article processed" [$__interval]
  ))
  ```
  Legend: `Consumed`

- **Draw style**: bars
- **Stack**: normal
- **Fill opacity**: 80
- **Point size**: 0

#### Panel 2.2: Error Rate by Service

- **Type**: timeseries
- **Width**: 10
- **Datasource**: Loki
- **Description**: Error log rate per pipeline service
- **Query**:
  ```logql
  sum by (service) (count_over_time(
    {service=~"crawler|classifier|publisher|source-manager|index-manager", level="error"} [$__interval]
  ))
  ```
  Legend: `{{service}}`
- **Draw style**: line
- **Fill opacity**: 10
- **Line width**: 2
- **Color scheme**: palette-classic
- **Thresholds**: none (relative scale)

---

### Row 3: Content Analytics (Elasticsearch)

Collapsed: no. Four panels from Elasticsearch aggregations.

#### Panel 3.1: Content Type Distribution

- **Type**: piechart
- **Width**: 6
- **Datasource**: Elasticsearch
- **Query**:
  ```json
  {
    "query": "*",
    "metrics": [{"type": "count", "id": "1"}],
    "bucketAggs": [
      {
        "type": "terms",
        "field": "content_type.keyword",
        "id": "2",
        "settings": {"size": "10", "order": "desc", "orderBy": "_count", "min_doc_count": "1"}
      }
    ],
    "timeField": "crawled_at"
  }
  ```
- **Pie type**: donut
- **Legend**: right, values
- **Tooltip**: single

#### Panel 3.2: Quality Score Distribution

- **Type**: histogram
- **Width**: 6
- **Datasource**: Elasticsearch
- **Description**: Distribution of quality scores across all classified content
- **Query**:
  ```json
  {
    "query": "content_type.keyword:article",
    "metrics": [{"type": "count", "id": "1"}],
    "bucketAggs": [
      {
        "type": "histogram",
        "field": "quality_score",
        "id": "2",
        "settings": {"interval": "10", "min_doc_count": "1"}
      }
    ],
    "timeField": "crawled_at"
  }
  ```
- **Fill opacity**: 80
- **Gradient mode**: scheme
- **Color scheme**: green-yellow-red (reversed — high quality = green)
- **X-axis label**: Quality Score

#### Panel 3.3: Crime Relevance Breakdown

- **Type**: piechart
- **Width**: 6
- **Datasource**: Elasticsearch
- **Query**:
  ```json
  {
    "query": "content_type.keyword:article",
    "metrics": [{"type": "count", "id": "1"}],
    "bucketAggs": [
      {
        "type": "terms",
        "field": "crime.street_crime_relevance.keyword",
        "id": "2",
        "settings": {"size": "10", "order": "desc", "orderBy": "_count", "min_doc_count": "1"}
      }
    ],
    "timeField": "crawled_at"
  }
  ```
- **Pie type**: donut
- **Legend**: right, values
- **Value mappings**: `core_street_crime` → "Core Crime", `peripheral_crime` → "Peripheral", `not_crime` → "Not Crime"

#### Panel 3.4: Mining Relevance Breakdown

- **Type**: piechart
- **Width**: 6
- **Datasource**: Elasticsearch
- **Query**:
  ```json
  {
    "query": "content_type.keyword:article AND _exists_:mining.relevance",
    "metrics": [{"type": "count", "id": "1"}],
    "bucketAggs": [
      {
        "type": "terms",
        "field": "mining.relevance.keyword",
        "id": "2",
        "settings": {"size": "10", "order": "desc", "orderBy": "_count", "min_doc_count": "1"}
      }
    ],
    "timeField": "crawled_at"
  }
  ```
- **Pie type**: donut
- **Legend**: right, values
- **Value mappings**: `core_mining` → "Core Mining", `peripheral_mining` → "Peripheral", `not_mining` → "Not Mining"

---

### Row 4: Publisher Routing

Collapsed: no. Two panels.

#### Panel 4.1: Articles per Redis Channel

- **Type**: barchart
- **Width**: 14
- **Datasource**: Loki
- **Description**: Which Redis channels received the most articles
- **Query**:
  ```logql
  sum by (channel) (count_over_time(
    {service="publisher"} |= "Published article to channel"
      | json channel="channel"
    [$__range]
  ))
  ```
  Legend: `{{channel}}`
- **Orientation**: horizontal
- **Sort by**: total descending
- **Bar width**: 0.6
- **Color scheme**: palette-classic
- **X-axis label**: Articles

#### Panel 4.2: Classifier Throughput Rate

- **Type**: timeseries
- **Width**: 10
- **Datasource**: Loki
- **Description**: Classifier processing speed (items/sec from batch complete logs)
- **Query**:
  ```logql
  avg_over_time(
    {service="classifier"} |= "[Processor] Batch processing complete"
      | json items_per_second="items_per_second"
      | unwrap items_per_second [$__interval]
  )
  ```
  Legend: `items/sec`
- **Draw style**: line
- **Fill opacity**: 20
- **Line width**: 2
- **Unit**: short (suffix: /s)

---

### Row 5: Suspected Misclassifications

Collapsed: **yes** (expandable).

#### Panel 5.1: Crime-Classified Pages

- **Type**: table
- **Width**: 24
- **Datasource**: Elasticsearch
- **Description**: Pages classified as crime content (likely misclassifications — pages are usually navigation, not articles)
- **Query**:
  ```json
  {
    "query": "content_type.keyword:page AND (crime.street_crime_relevance.keyword:core_street_crime OR crime.street_crime_relevance.keyword:peripheral_crime)",
    "metrics": [{"type": "raw_data", "id": "1", "settings": {"size": "50"}}],
    "bucketAggs": [],
    "timeField": "crawled_at"
  }
  ```
- **Column overrides**:
  - Show: `title`, `url`, `source_name`, `content_type`, `crime.street_crime_relevance`, `quality_score`, `crawled_at`
  - Hide all other columns
  - `url`: render as link
  - `crawled_at`: format as datetime

---

### Row 6: Recent Pipeline Errors

Collapsed: **yes**.

#### Panel 6.1: Error Stream

- **Type**: logs
- **Width**: 24
- **Datasource**: Loki
- **Query**:
  ```logql
  {service=~"crawler|classifier|publisher|source-manager|index-manager", level="error"}
    |= "" | json
    | line_format "[{{.service}}] {{.msg}} {{if .error}}err={{.error}}{{end}} {{if .err}}err={{.err}}{{end}}"
  ```
- **Sort order**: newest first
- **Dedup**: signature
- **Enable log details**: true
- **Pretty JSON**: true

---

## Dashboard 2: Deployer Sites

**UID**: `north-cloud-deployer-sites`
**Title**: Deployer Sites
**Default time range**: Last 6 hours
**Refresh**: 30s
**Tags**: `north-cloud`, `deployer`, `laravel`

### Variables

#### `site`

- **Type**: custom multi-select
- **Values**: `streetcode`, `orewire`, `coforge`, `movies-of-war`
- **Default**: All selected
- **Label**: Site

#### `level`

- **Type**: custom multi-select
- **Values**: `debug`, `info`, `notice`, `warning`, `error`, `critical`, `alert`, `emergency`
- **Default**: `info,warning,error,critical`
- **Label**: Level

---

### Row 1: Site Health Overview

Collapsed: no. Four columns of stat panels (one per site).

For each site in `[streetcode, orewire, coforge, movies-of-war]`, create 3 stat panels (12 total):

#### Panel pattern: `{site} — Log Volume`

- **Type**: stat
- **Width**: 6
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time({service="{site}"} [$__range])
  ```
- **Unit**: short
- **Color mode**: background, blue
- **Graph mode**: area

#### Panel pattern: `{site} — Errors`

- **Type**: stat
- **Width**: 6
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time({service="{site}", level="error"} [$__range])
  ```
- **Unit**: short
- **Color mode**: background
- **Thresholds**: 0 = green, 1 = yellow, 10 = red
- **Graph mode**: area

#### Panel pattern: `{site} — Last Log`

- **Type**: stat
- **Width**: 6
- **Datasource**: Loki
- **Description**: Detects if site has gone silent
- **Query (instant)**:
  ```logql
  max_over_time(
    {service="{site}"} | unwrap __timestamp__ [$__range]
  ) / 1e9
  ```
  Note: If this doesn't work in Grafana's Loki plugin, use a table panel with `{service="{site}"} | limit 1` and display the timestamp.
- **Unit**: dateTimeFromNow
- **Color mode**: background
- **Thresholds**: time-based (recent = green, >30min = yellow, >2h = red)

**Layout**: Stack 3 stat panels per site in a 6-wide column. Four columns = 24 total.
Actual grid: Each site gets a vertical group of 3 panels at w=6.

---

### Row 2: Error Trends

Collapsed: no.

#### Panel 2.1: Error Rate by Site

- **Type**: timeseries
- **Width**: 24
- **Datasource**: Loki
- **Query**:
  ```logql
  sum by (service) (count_over_time(
    {service=~"streetcode|orewire|coforge|movies-of-war", level="error"} [$__interval]
  ))
  ```
  Legend: `{{service}}`
- **Draw style**: line
- **Line width**: 2
- **Fill opacity**: 10
- **Point size**: 5
- **Color scheme**: palette-classic

---

### Row 3: Ingestion Metrics

Collapsed: no. Stat panels for sites that consume North Cloud articles.

#### Panel 3.1: Streetcode — Articles Ingested

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="streetcode"} |= "Article processed" [$__range]
  )
  ```
- **Color mode**: background, green
- **Graph mode**: area

#### Panel 3.2: Streetcode — Skipped Non-Core

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="streetcode"} |= "Skipping non-core-crime" [$__range]
  )
  ```
- **Color mode**: background, yellow
- **Graph mode**: area

#### Panel 3.3: Orewire — Articles Ingested

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="orewire"} |= "Article processed" [$__range]
  )
  ```
- **Color mode**: background, green
- **Graph mode**: area

#### Panel 3.4: Orewire — Duplicates Skipped

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="orewire"} |= "Skipping duplicate" [$__range]
  )
  ```
- **Color mode**: background, yellow
- **Graph mode**: area

#### Panel 3.5: Coforge — Articles Ingested

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="coforge"} |= "Article processed" [$__range]
  )
  ```
- **Color mode**: background, green
- **Graph mode**: area

#### Panel 3.6: Coforge — Skipped

- **Type**: stat
- **Width**: 4
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  count_over_time(
    {service="coforge"} |= "Skipping" [$__range]
  )
  ```
- **Color mode**: background, yellow
- **Graph mode**: area

---

### Rows 4–7: Per-Site Log Streams

One collapsible row per site. Each row contains one logs panel.

#### Row 4: Streetcode Logs (collapsed: yes)

- **Panel**: Streetcode Log Stream
- **Type**: logs
- **Width**: 24
- **Datasource**: Loki
- **Query**:
  ```logql
  {service="streetcode", level=~"$level"}
  ```
- **Sort**: newest first
- **Enable log details**: true

#### Row 5: Orewire Logs (collapsed: yes)

- **Panel**: Orewire Log Stream
- **Type**: logs
- **Width**: 24
- **Query**:
  ```logql
  {service="orewire", level=~"$level"}
  ```

#### Row 6: Coforge Logs (collapsed: yes)

- **Panel**: Coforge Log Stream
- **Type**: logs
- **Width**: 24
- **Query**:
  ```logql
  {service="coforge", level=~"$level"}
  ```

#### Row 7: Movies of War Logs (collapsed: yes)

- **Panel**: Movies of War Log Stream
- **Type**: logs
- **Width**: 24
- **Query**:
  ```logql
  {service="movies-of-war", level=~"$level"}
  ```

---

## Dashboard 3: Service Logs

**UID**: `north-cloud-service-logs`
**Title**: Service Logs
**Default time range**: Last 1 hour
**Refresh**: 10s
**Tags**: `north-cloud`, `logs`

### Variables

#### `service`

- **Type**: query (Loki label values)
- **Query**: `label_values({project="north-cloud"}, service)`
- **Multi-select**: yes
- **Include all**: yes
- **Default**: All
- **Label**: Service

#### `level`

- **Type**: custom multi-select
- **Values**: `debug`, `info`, `warn`, `error`
- **Default**: `info,warn,error`
- **Label**: Level

#### `search`

- **Type**: textbox
- **Default**: (empty)
- **Label**: Search

---

### Row 1: Log Volume

Collapsed: no.

#### Panel 1.1: Log Volume by Service

- **Type**: timeseries
- **Width**: 16
- **Datasource**: Loki
- **Query**:
  ```logql
  sum by (service) (count_over_time(
    {project="north-cloud", service=~"$service", level=~"$level"} [$__interval]
  ))
  ```
  Legend: `{{service}}`
- **Draw style**: bars
- **Stack**: normal
- **Fill opacity**: 80
- **Color scheme**: palette-classic

#### Panel 1.2: Log Volume by Level

- **Type**: timeseries
- **Width**: 8
- **Datasource**: Loki
- **Query**:
  ```logql
  sum by (level) (count_over_time(
    {project="north-cloud", service=~"$service", level=~"$level"} [$__interval]
  ))
  ```
  Legend: `{{level}}`
- **Draw style**: bars
- **Stack**: normal
- **Fill opacity**: 80
- **Color overrides**: debug = blue, info = green, warn = yellow, error = red

---

### Row 2: Error Summary

Collapsed: no.

#### Panel 2.1: Errors by Service (Last 5m)

- **Type**: bargauge
- **Width**: 8
- **Datasource**: Loki
- **Query (instant)**:
  ```logql
  sum by (service) (count_over_time(
    {project="north-cloud", level="error"} [5m]
  ))
  ```
  Legend: `{{service}}`
- **Orientation**: horizontal
- **Thresholds**: 0 = green, 5 = yellow, 20 = red
- **Display mode**: gradient

#### Panel 2.2: Top Error Messages

- **Type**: table
- **Width**: 16
- **Datasource**: Loki
- **Description**: Most frequent error messages in the selected time range
- **Query (instant)**:
  ```logql
  topk(15,
    sum by (service, msg) (count_over_time(
      {project="north-cloud", service=~"$service", level="error"}
        | json msg="msg"
      [$__range]
    ))
  )
  ```
- **Transform**: Labels to fields
- **Column overrides**:
  - `service`: width 120
  - `msg`: width auto (fill)
  - `Value`: rename to "Count", width 80

---

### Row 3: Log Stream

Collapsed: no.

#### Panel 3.1: Logs

- **Type**: logs
- **Width**: 24
- **Datasource**: Loki
- **Query**:
  ```logql
  {project="north-cloud", service=~"$service", level=~"$level"} |~ "$search"
    | json
    | line_format "{{if .service}}[{{.service}}]{{end}} {{.level}} | {{.msg}} {{if .method}}{{.method}} {{.path}} {{.status}}{{end}} {{if .duration}}({{.duration}}){{end}} {{if .error}}err={{.error}}{{end}} {{if .err}}err={{.err}}{{end}} {{if .caller}}← {{.caller}}{{end}}"
  ```
- **Sort**: newest first
- **Dedup**: signature
- **Enable log details**: true
- **Pretty JSON**: true
- **Wrap log lines**: true

---

## Alerting Rules

Provisioned via `/infrastructure/grafana/provisioning/alerting/alerts.yml`.

### Alert 1: Pipeline Stall

- **Name**: Pipeline stall — no classifications in 2 hours
- **Condition**: Classification count = 0 over 2h window
- **Query**:
  ```logql
  count_over_time(
    {service="classifier"} |= "[Processor] Classification complete" [2h]
  )
  ```
- **Threshold**: `< 1` → firing
- **For**: 0m (fire immediately — 2h is already the evaluation window)
- **Labels**: `severity: critical`, `service: classifier`
- **Annotations**:
  - Summary: "No articles classified in the last 2 hours"
  - Description: "The classifier has not produced any classification results. Check classifier, Elasticsearch, and raw content availability."

### Alert 2: Error Spike

- **Name**: Error spike — sustained errors across pipeline
- **Condition**: Error rate > 10/min for 5 minutes
- **Query**:
  ```logql
  sum(rate(
    {service=~"crawler|classifier|publisher", level="error"} [5m]
  ))
  ```
- **Threshold**: `> 0.167` (10/min = 0.167/sec) → firing
- **For**: 5m
- **Labels**: `severity: warning`, `team: platform`
- **Annotations**:
  - Summary: "Elevated error rate in pipeline services"
  - Description: "More than 10 errors/minute sustained for 5 minutes across pipeline services."

### Alert 3: Deployer Site Silence

- **Name**: Deployer site silence — no logs from a consumer
- **Condition**: No log entries from a deployer site in 30 minutes
- **Query** (one per site — `streetcode`, `orewire`, `coforge`):
  ```logql
  count_over_time({service="streetcode"} [30m])
  ```
- **Threshold**: `< 1` → firing
- **For**: 0m
- **Labels**: `severity: warning`, `service: streetcode`
- **Annotations**:
  - Summary: "{{ $labels.service }} has not produced logs in 30 minutes"
  - Description: "The Laravel consumer may be down or the log shipping pipeline is broken."

### Alert 4: Publisher Zero Output

- **Name**: Publisher zero output — no articles published in 4 hours
- **Query**:
  ```logql
  count_over_time(
    {service="publisher"} |= "Published article to channel" [4h]
  )
  ```
- **Threshold**: `< 1` → firing
- **For**: 0m
- **Labels**: `severity: critical`, `service: publisher`
- **Annotations**:
  - Summary: "No articles published in 4 hours"
  - Description: "Publisher has not pushed any articles to Redis. Check source availability, classifier output, and publisher routing configuration."

---

## Dashboard Links

Each dashboard includes a top-level links bar for cross-navigation:

```json
"links": [
  {"title": "Pipeline Ops", "url": "/d/north-cloud-pipeline-ops", "type": "link", "icon": "dashboard"},
  {"title": "Deployer Sites", "url": "/d/north-cloud-deployer-sites", "type": "link", "icon": "dashboard"},
  {"title": "Service Logs", "url": "/d/north-cloud-service-logs", "type": "link", "icon": "dashboard"},
  {"title": "Explore Loki", "url": "/explore?orgId=1&left=%7B%22datasource%22:%22loki%22%7D", "type": "link", "icon": "search"}
]
```

---

## Implementation Notes

### Files to Create/Modify

| File | Action |
|------|--------|
| `infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline-ops.json` | Create |
| `infrastructure/grafana/provisioning/dashboards/north-cloud-deployer-sites.json` | Create |
| `infrastructure/grafana/provisioning/dashboards/north-cloud-service-logs.json` | Create |
| `infrastructure/grafana/provisioning/dashboards/north-cloud-logs.json` | Delete |
| `infrastructure/grafana/provisioning/dashboards/north-cloud-pipeline.json` | Delete |
| `infrastructure/grafana/provisioning/alerting/alerts.yml` | Create |

### ES Keyword Field Gotcha

All Elasticsearch aggregations MUST use `.keyword` suffix on text fields due to dynamic mapping drift:
- `content_type.keyword` (not `content_type`)
- `crime.street_crime_relevance.keyword`
- `mining.relevance.keyword`
- `source_name.keyword`

### Laravel Level Labels

The `loki.process "laravel"` pipeline lowercases levels via `stage.template`. Laravel levels `ERROR`, `INFO` etc. become `error`, `info` in Loki labels. Dashboard queries should use lowercase.

### Grafana Provisioning Reload

Grafana checks provisioning directory every 10 seconds (`updateIntervalSeconds: 10`). New dashboard files are auto-loaded. Deleted files remove dashboards if `disableDeletion: false`.

### Panel IDs

Each panel in Grafana JSON needs a unique integer `id`. Use sequential IDs starting from 1 within each dashboard.

### Grid Layout

Grafana uses a 24-column grid. Panel positions use `gridPos: {x, y, w, h}`.
- Row panels: `{x:0, y:Y, w:24, h:1}`
- Stat panels: typically `h:4`
- Time series: typically `h:8`
- Tables: typically `h:10`
- Logs: typically `h:12`
- Pie charts: typically `h:8`

# Elasticsearch Single-Node Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Optimize the single ES 9.2.2 node on razor-crest by increasing resource allocation, adding ILM for raw_content cleanup, setting replicas to 0, and tuning index settings.

**Architecture:** No new infrastructure. Update Docker resource limits and ES heap to use the 16GB droplet's available headroom. Add ILM policies via ES API. Update index-manager defaults to replicas=0 for single-node deployment. Add a startup script for ILM and index tuning.

**Tech Stack:** Docker Compose, Elasticsearch 9.2.2, Go (index-manager), shell scripts

---

### Task 1: Update Production Docker Compose ES Limits

**Files:**
- Modify: `docker-compose.prod.yml:502-511`

**Step 1: Update ES resource limits and heap**

In `docker-compose.prod.yml`, change the elasticsearch prod overrides section:

```yaml
  # ============================================================
  # Elasticsearch (Prod Overrides)
  # ============================================================
  elasticsearch:
    deploy:
      resources:
        limits:
          cpus: "4"
          memory: 8G
    shm_size: 512mb
    environment:
      xpack.security.enabled: "${ELASTICSEARCH_SECURITY_ENABLED:-false}"
      ES_JAVA_OPTS: "-Xms${ELASTICSEARCH_MIN_HEAP:-4g} -Xmx${ELASTICSEARCH_MAX_HEAP:-4g}"
```

Changes: `cpus: "2"` -> `"4"`, `memory: 2G` -> `8G`, heap defaults `256m/512m` -> `4g/4g`.

**Step 2: Verify compose config parses**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.prod.yml config --services`
Expected: List of services with no errors

**Step 3: Commit**

```bash
git add docker-compose.prod.yml
git commit -m "perf(elasticsearch): increase prod container limits to 8G RAM, 4 CPUs, 4g heap"
```

---

### Task 2: Update Production .env and .env.example

**Files:**
- Modify: Production `.env` on razor-crest (via SSH at `jones@147.182.150.145:/opt/north-cloud/.env` or wherever it lives)
- Modify: `.env.example`

**Step 1: Update .env.example with recommended prod values**

Change the Elasticsearch section in `.env.example`:

```
# Elasticsearch
ELASTICSEARCH_SECURITY_ENABLED=false
ELASTICSEARCH_URL=http://localhost:9200
# Production recommended: 4g (50% of container memory limit)
# ELASTICSEARCH_MIN_HEAP=4g
# ELASTICSEARCH_MAX_HEAP=4g
```

**Step 2: Update production .env via SSH**

```bash
ssh jones@147.182.150.145 "cd /opt/north-cloud && sed -i 's/ELASTICSEARCH_MIN_HEAP=1g/ELASTICSEARCH_MIN_HEAP=4g/' .env && sed -i 's/ELASTICSEARCH_MAX_HEAP=1g/ELASTICSEARCH_MAX_HEAP=4g/' .env && grep ELASTICSEARCH .env"
```

Expected output should show `ELASTICSEARCH_MIN_HEAP=4g` and `ELASTICSEARCH_MAX_HEAP=4g`.

Note: If the .env is not at `/opt/north-cloud/.env`, find it first:
```bash
ssh jones@147.182.150.145 "find /home/jones /opt -name '.env' -path '*/north-cloud/*' 2>/dev/null"
```

**Step 3: Commit local changes**

```bash
git add .env.example
git commit -m "docs: update .env.example with recommended ES heap values for production"
```

---

### Task 3: Update index-manager Default Replicas to 0

**Files:**
- Modify: `index-manager/internal/config/config.go:28`
- Modify: `index-manager/internal/elasticsearch/mappings/mappings.go:17`
- Modify: `index-manager/internal/elasticsearch/mappings/mappings_test.go:19-20`

**Step 1: Update the test to expect replicas=0**

In `index-manager/internal/elasticsearch/mappings/mappings_test.go`, change:

```go
	if settings.NumberOfReplicas != 0 {
		t.Errorf("NumberOfReplicas = %d, want 0", settings.NumberOfReplicas)
	}
```

**Step 2: Run test to verify it fails**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -run TestDefaultSettings -v`
Expected: FAIL with `NumberOfReplicas = 1, want 0`

**Step 3: Update config.go default constant**

In `index-manager/internal/config/config.go`, change:

```go
	defaultReplicas        = 0
```

**Step 4: Update mappings.go DefaultSettings**

In `index-manager/internal/elasticsearch/mappings/mappings.go`, change:

```go
func DefaultSettings() BaseSettings {
	return BaseSettings{
		NumberOfShards:   1,
		NumberOfReplicas: 0,
	}
}
```

**Step 5: Remove the classified_content replicas default override**

In `index-manager/internal/config/config.go`, the `setIndexTypeDefaults` function sets replicas=1 for classified_content when it's 0. Remove that override since 0 is now correct for single-node:

Change `setIndexTypeDefaults` to:

```go
func setIndexTypeDefaults(cfg *IndexTypesConfig) {
	if cfg.RawContent.Shards == 0 {
		cfg.RawContent.Shards = defaultShards
	}
	// raw_content replicas default 0 (transient, rebuildable from source)
	// No special handling needed since Go zero-value is 0

	if cfg.ClassifiedContent.Shards == 0 {
		cfg.ClassifiedContent.Shards = defaultShards
	}
	// classified_content replicas default 0 (single-node: replicas provide no redundancy)
	// No special handling needed since Go zero-value is 0
}
```

**Step 6: Run tests to verify they pass**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 7: Run linter**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 8: Commit**

```bash
git add index-manager/internal/config/config.go index-manager/internal/elasticsearch/mappings/mappings.go index-manager/internal/elasticsearch/mappings/mappings_test.go
git commit -m "perf(index-manager): set default replicas to 0 for single-node ES deployment"
```

---

### Task 4: Add Index Tuning to elasticsearch.yml

**Files:**
- Modify: `infrastructure/elasticsearch/elasticsearch.yml`

**Step 1: Add index buffer size setting**

Add to `infrastructure/elasticsearch/elasticsearch.yml` before the logging section:

```yaml
# Index buffer (default 10%, increase for write-heavy workloads)
indices.memory.index_buffer_size: 20%
```

**Step 2: Verify config is valid YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('infrastructure/elasticsearch/elasticsearch.yml'))"`
Expected: No errors

**Step 3: Commit**

```bash
git add infrastructure/elasticsearch/elasticsearch.yml
git commit -m "perf(elasticsearch): increase index buffer size to 20%"
```

---

### Task 5: Create ILM and Index Tuning Bootstrap Script

**Files:**
- Create: `infrastructure/elasticsearch/setup-ilm.sh`

This script applies ILM policies and tunes existing indexes via the ES API. It's idempotent and safe to re-run.

**Step 1: Write the script**

Create `infrastructure/elasticsearch/setup-ilm.sh`:

```bash
#!/usr/bin/env bash
# Setup Elasticsearch ILM policies and tune existing indexes.
# Idempotent - safe to re-run.
#
# Usage: ./setup-ilm.sh [ES_URL]
# Default ES_URL: http://localhost:9200

set -euo pipefail

ES_URL="${1:-http://localhost:9200}"

echo "==> Elasticsearch ILM Setup (${ES_URL})"

# 1. Create ILM policy: delete raw_content after 30 days
echo "--- Creating raw_content ILM policy (30-day delete)..."
curl -s -X PUT "${ES_URL}/_ilm/policy/raw_content_cleanup" \
  -H 'Content-Type: application/json' \
  -d '{
  "policy": {
    "phases": {
      "hot": {
        "actions": {
          "rollover": {
            "max_age": "30d"
          }
        }
      },
      "delete": {
        "min_age": "30d",
        "actions": {
          "delete": {}
        }
      }
    }
  }
}'
echo ""

# 2. Set replicas=0 on all existing classified_content indexes
echo "--- Setting replicas=0 on classified_content indexes..."
CLASSIFIED_INDEXES=$(curl -s "${ES_URL}/_cat/indices/*_classified_content?h=index" | tr -d '[:space:]' | tr '\n' ',')
if [ -n "$CLASSIFIED_INDEXES" ]; then
  curl -s -X PUT "${ES_URL}/${CLASSIFIED_INDEXES%,}/_settings" \
    -H 'Content-Type: application/json' \
    -d '{"index": {"number_of_replicas": 0}}'
  echo ""
fi

# 3. Set refresh_interval=30s on all raw_content indexes
echo "--- Setting refresh_interval=30s on raw_content indexes..."
RAW_INDEXES=$(curl -s "${ES_URL}/_cat/indices/*_raw_content?h=index" | tr -d '[:space:]' | tr '\n' ',')
if [ -n "$RAW_INDEXES" ]; then
  curl -s -X PUT "${ES_URL}/${RAW_INDEXES%,}/_settings" \
    -H 'Content-Type: application/json' \
    -d '{"index": {"refresh_interval": "30s"}}'
  echo ""
fi

# 4. Force-merge indexes with more than 1 segment (read-only or low-write)
echo "--- Force-merging classified_content indexes..."
if [ -n "$CLASSIFIED_INDEXES" ]; then
  curl -s -X POST "${ES_URL}/${CLASSIFIED_INDEXES%,}/_forcemerge?max_num_segments=1" &>/dev/null || true
  echo "Force-merge initiated (runs async)"
fi

echo "==> ILM setup complete"
```

**Step 2: Make it executable**

Run: `chmod +x infrastructure/elasticsearch/setup-ilm.sh`

**Step 3: Commit**

```bash
git add infrastructure/elasticsearch/setup-ilm.sh
git commit -m "feat(elasticsearch): add ILM setup script for raw_content cleanup and index tuning"
```

---

### Task 6: Deploy and Verify

This task runs on production (razor-crest). Do NOT commit anything here — this is operational.

**Step 1: Pull latest changes on production**

```bash
ssh jones@147.182.150.145 "cd /opt/north-cloud && git pull"
```

(Adjust path if north-cloud is elsewhere on the server.)

**Step 2: Restart ES container with new limits**

```bash
ssh jones@147.182.150.145 "cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d elasticsearch"
```

**Step 3: Wait for ES to become healthy**

```bash
ssh jones@147.182.150.145 "until curl -sf http://localhost:9200/_cluster/health?wait_for_status=yellow; do sleep 5; done"
```

Expected: `{"status":"green",...}` or `{"status":"yellow",...}`

**Step 4: Verify new heap allocation**

```bash
ssh jones@147.182.150.145 "curl -s http://localhost:9200/_nodes/stats/jvm | python3 -c 'import sys,json; d=json.load(sys.stdin); [print(f\"Heap max: {n[\"jvm\"][\"mem\"][\"heap_max_in_bytes\"]/(1024**3):.1f}GB\") for n in d[\"nodes\"].values()]'"
```

Expected: `Heap max: 4.0GB`

**Step 5: Run ILM setup script**

```bash
ssh jones@147.182.150.145 "cd /opt/north-cloud && bash infrastructure/elasticsearch/setup-ilm.sh http://localhost:9200"
```

Expected: All steps complete with `200 OK` responses

**Step 6: Verify ILM policy exists**

```bash
ssh jones@147.182.150.145 "curl -s http://localhost:9200/_ilm/policy/raw_content_cleanup | python3 -m json.tool"
```

Expected: JSON showing the policy with `"max_age": "30d"` and delete phase

**Step 7: Verify replicas=0 on classified indexes**

```bash
ssh jones@147.182.150.145 "curl -s 'http://localhost:9200/_cat/indices/*_classified_content?v&h=index,rep,docs.count,store.size'"
```

Expected: All indexes showing `rep=0`

**Step 8: Check overall cluster health**

```bash
ssh jones@147.182.150.145 "curl -s http://localhost:9200/_cluster/health?pretty"
```

Expected: `"status": "green"` (since replicas=0, no unassigned shards)

**Step 9: Verify memory usage looks healthy**

```bash
ssh jones@147.182.150.145 "free -h && echo '---' && docker stats --no-stream --format 'table {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}' | grep -E 'NAME|elastic'"
```

Expected: ES container using ~4-5GB, total system memory still has headroom

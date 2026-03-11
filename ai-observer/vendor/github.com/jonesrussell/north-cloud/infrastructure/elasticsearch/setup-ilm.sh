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

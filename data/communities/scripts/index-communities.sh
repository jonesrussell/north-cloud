#!/usr/bin/env bash
#
# Bulk-index communities into Elasticsearch.
#
# Usage:
#   ./scripts/index-communities.sh                              # defaults to localhost:9200
#   ./scripts/index-communities.sh https://es.example.com:9200  # custom ES URL
#   COMMUNITIES_INDEX=my_communities ./scripts/index-communities.sh
#
set -euo pipefail

ES_URL="${1:-http://localhost:9200}"
INDEX_NAME="${COMMUNITIES_INDEX:-northcloud_communities}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_DIR="${SCRIPT_DIR}/.."
NDJSON_FILE="${DATA_DIR}/communities.ndjson"
MAPPING_FILE="${DATA_DIR}/es-mapping.json"

if [ ! -f "$NDJSON_FILE" ]; then
    echo "ERROR: communities.ndjson not found at ${NDJSON_FILE}"
    exit 1
fi

echo "==> Elasticsearch: ${ES_URL}"
echo "==> Index:          ${INDEX_NAME}"
echo "==> Data:           ${NDJSON_FILE}"

# Check ES connectivity
if ! curl -sf "${ES_URL}/_cluster/health" > /dev/null 2>&1; then
    echo "ERROR: Cannot reach Elasticsearch at ${ES_URL}"
    exit 1
fi

# Delete existing index if present
if curl -sf "${ES_URL}/${INDEX_NAME}" > /dev/null 2>&1; then
    echo "==> Deleting existing index ${INDEX_NAME}..."
    curl -sf -X DELETE "${ES_URL}/${INDEX_NAME}" > /dev/null
fi

# Create index with mapping
echo "==> Creating index ${INDEX_NAME} with mapping..."
if [ -f "$MAPPING_FILE" ]; then
    curl -sf -X PUT "${ES_URL}/${INDEX_NAME}" \
        -H "Content-Type: application/json" \
        -d @"${MAPPING_FILE}" > /dev/null
else
    echo "WARNING: es-mapping.json not found, creating index without explicit mapping"
    curl -sf -X PUT "${ES_URL}/${INDEX_NAME}" > /dev/null
fi

# Build bulk request body
# Each document needs: {"index":{"_id":"<id>"}}\n{doc}\n
BULK_BODY=$(mktemp)
trap 'rm -f "$BULK_BODY"' EXIT

while IFS= read -r line; do
    id=$(echo "$line" | jq -r '.id')
    # Transform for ES: rename type→community_type, lat/lon→location geo_point
    doc=$(echo "$line" | jq '{
        id: .id,
        name: .name,
        community_type: .type,
        province: .province,
        location: { lat: .latitude, lon: .longitude },
        population: .population,
        governing_body: .governing_body,
        external_ids: .external_ids,
        region: .region,
        neighbours: .neighbours,
        notes: .notes
    }')
    printf '{"index":{"_id":"%s"}}\n%s\n' "$id" "$doc" >> "$BULK_BODY"
done < "$NDJSON_FILE"

# Bulk index
echo "==> Bulk indexing communities..."
RESPONSE=$(curl -sf -X POST "${ES_URL}/${INDEX_NAME}/_bulk" \
    -H "Content-Type: application/x-ndjson" \
    --data-binary @"${BULK_BODY}")

ERRORS=$(echo "$RESPONSE" | jq '.errors')
if [ "$ERRORS" = "true" ]; then
    echo "ERROR: Some documents failed to index:"
    echo "$RESPONSE" | jq '.items[] | select(.index.error) | .index.error'
    exit 1
fi

INDEXED=$(echo "$RESPONSE" | jq '[.items[] | select(.index.result)] | length')
echo "==> Successfully indexed ${INDEXED} communities into ${INDEX_NAME}"

# Refresh index
curl -sf -X POST "${ES_URL}/${INDEX_NAME}/_refresh" > /dev/null
echo "==> Index refreshed"

# Verify
COUNT=$(curl -sf "${ES_URL}/${INDEX_NAME}/_count" | jq '.count')
echo "==> Verified: ${COUNT} documents in ${INDEX_NAME}"

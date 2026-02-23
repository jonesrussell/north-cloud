#!/bin/bash
# Add Anishinaabe/Indigenous sources to North Cloud and schedule recurring crawls.
# Use on production (e.g. jones@northcloud.biz) to increase content for diidjaaheer.live.
#
# Usage:
#   ./scripts/add-anishinaabe-sources.sh [path/to/anishinaabe-sources-data.json]
#
# Default data file: scripts/anishinaabe-sources-data.json (relative to repo root).
#
# Environment:
#   AUTH_URL            - Auth base URL (default: http://localhost:8040). Login path /api/v1/auth/login appended.
#   SOURCE_MANAGER_URL  - Source manager API (default: http://localhost:8050).
#   CRAWLER_URL         - Crawler API (default: http://localhost:8060).
#   JWT                 - JWT token (optional; if unset, use AUTH_USERNAME + AUTH_PASSWORD to fetch).
#   AUTH_USERNAME       - Username for login (required if JWT not set).
#   AUTH_PASSWORD       - Password for login (required if JWT not set).
#   INTERVAL_MINUTES    - Crawl interval in minutes (default: 360 = 6 hours).
#   INTERVAL_TYPE       - minutes | hours | days (default: minutes).
#   DRY_RUN             - If non-empty, only print what would be done (no API calls).
#
# Example (production, SSH to server):
#   cd /opt/north-cloud
#   export AUTH_USERNAME="admin"
#   export AUTH_PASSWORD="your-password"
#   export AUTH_URL="http://localhost:8040"
#   export SOURCE_MANAGER_URL="http://localhost:8050"
#   export CRAWLER_URL="http://localhost:8060"
#   ./scripts/add-anishinaabe-sources.sh
#
# Example (dry run):
#   DRY_RUN=1 AUTH_USERNAME=admin AUTH_PASSWORD=x ./scripts/add-anishinaabe-sources.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DATA_FILE="${1:-$SCRIPT_DIR/anishinaabe-sources-data.json}"

AUTH_URL="${AUTH_URL:-http://localhost:8040}"
SOURCE_MANAGER_URL="${SOURCE_MANAGER_URL:-http://localhost:8050}"
CRAWLER_URL="${CRAWLER_URL:-http://localhost:8060}"
INTERVAL_MINUTES="${INTERVAL_MINUTES:-360}"
INTERVAL_TYPE="${INTERVAL_TYPE:-minutes}"

# Resolve data file path
if [ ! -f "$DATA_FILE" ]; then
  if [ -f "$REPO_ROOT/$DATA_FILE" ]; then
    DATA_FILE="$REPO_ROOT/$DATA_FILE"
  else
    echo "Error: Source data file not found: $DATA_FILE" >&2
    exit 1
  fi
fi

# Fetch JWT if not set (skip when DRY_RUN and no credentials)
if [ -z "${JWT:-}" ] && [ -n "${AUTH_USERNAME:-}" ] && [ -n "${AUTH_PASSWORD:-}" ]; then
  LOGIN_URL="${AUTH_URL}/api/v1/auth/login"
  LOGIN_RESPONSE="$(curl -s -X POST "$LOGIN_URL" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${AUTH_USERNAME}\",\"password\":\"${AUTH_PASSWORD}\"}")"
  JWT="$(echo "$LOGIN_RESPONSE" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
  if [ -z "$JWT" ]; then
    echo "Error: Failed to get JWT from $LOGIN_URL (check AUTH_USERNAME, AUTH_PASSWORD)." >&2
    exit 1
  fi
fi

if [ -z "${JWT:-}" ] && [ -z "${DRY_RUN:-}" ]; then
  echo "Error: Set JWT or AUTH_USERNAME + AUTH_PASSWORD (or use DRY_RUN=1 to only list)." >&2
  exit 1
fi

# Default selectors (WordPress-style; override per source if needed)
SELECTORS='{"article":{"title":"h1","body":"article","published_time":"time[datetime]"},"list":{},"page":{}}'

# Count entries in JSON array
count=$(jq -r 'length' "$DATA_FILE")
echo "Adding $count sources from $DATA_FILE ..." >&2

added=0
skipped=0
errors=0

for i in $(seq 0 $((count - 1))); do
  name=$(jq -r ".[$i].name" "$DATA_FILE")
  url=$(jq -r ".[$i].url" "$DATA_FILE")
  feed_url=$(jq -r ".[$i].feed_url // empty" "$DATA_FILE")

  if [ -z "$name" ] || [ -z "$url" ] || [ "$name" = "null" ] || [ "$url" = "null" ]; then
    echo "Skipping row $i: missing name or url" >&2
    ((skipped++)) || true
    continue
  fi

  if [ -n "${DRY_RUN:-}" ]; then
    echo "[DRY RUN] Would add: $name | $url" >&2
    ((added++)) || true
    continue
  fi

  # Build source payload for source-manager (matches models.Source)
  source_payload=$(jq -n \
    --arg name "$name" \
    --arg url "$url" \
    --argjson selectors "$SELECTORS" \
    --arg feed_url "$feed_url" \
    '{
      name: $name,
      url: $url,
      rate_limit: "10",
      max_depth: 3,
      time: [],
      selectors: $selectors,
      enabled: true,
      ingestion_mode: "spider"
    } + (if $feed_url != "" then { feed_url: $feed_url } else {} end)')

  resp=$(curl -s -w "\n%{http_code}" -X POST "${SOURCE_MANAGER_URL}/api/v1/sources" \
    -H "Authorization: Bearer $JWT" \
    -H "Content-Type: application/json" \
    -d "$source_payload")
  http_code=$(echo "$resp" | tail -n1)
  body=$(echo "$resp" | sed '$d')

  if [ "$http_code" != "201" ] && [ "$http_code" != "200" ]; then
    echo "Error adding source '$name': HTTP $http_code — $body" >&2
    ((errors++)) || true
    continue
  fi

  source_id=$(echo "$body" | jq -r '.id')
  if [ -z "$source_id" ] || [ "$source_id" = "null" ]; then
    echo "Error: No id in response for '$name'" >&2
    ((errors++)) || true
    continue
  fi

  job_payload=$(jq -n \
    --arg source_id "$source_id" \
    --arg url "$url" \
    --argjson interval_minutes "$INTERVAL_MINUTES" \
    --arg interval_type "$INTERVAL_TYPE" \
    '{
      source_id: $source_id,
      url: $url,
      schedule_enabled: true,
      interval_minutes: $interval_minutes,
      interval_type: $interval_type
    }')

  job_resp=$(curl -s -w "\n%{http_code}" -X POST "${CRAWLER_URL}/api/v1/jobs" \
    -H "Authorization: Bearer $JWT" \
    -H "Content-Type: application/json" \
    -d "$job_payload")
  job_http_code=$(echo "$job_resp" | tail -n1)
  job_body=$(echo "$job_resp" | sed '$d')

  if [ "$job_http_code" != "201" ] && [ "$job_http_code" != "200" ]; then
    echo "Error scheduling crawl for '$name' (source_id=$source_id): HTTP $job_http_code — $job_body" >&2
    ((errors++)) || true
    continue
  fi

  echo "Added and scheduled: $name ($source_id)" >&2
  ((added++)) || true
done

echo "Done: $added added, $skipped skipped, $errors errors." >&2
[ "$errors" -eq 0 ] || exit 1

#!/bin/bash
# Sync enabled sources to crawler jobs (production-safe, idempotent).
# Ensures every enabled source in source-manager has a scheduled crawler job.
#
# Usage: ./scripts/sync-enabled-sources-jobs.sh
#
# Environment:
#   CRAWLER_URL   - Crawler API base URL (default: http://localhost:8060).
#                   On prod behind nginx: https://northcloud.biz/api/crawler
#   JWT           - JWT token for /api/v1 (required unless AUTH_* are set)
#   AUTH_URL      - Auth login URL (optional; if set with AUTH_USERNAME/AUTH_PASSWORD, JWT is fetched)
#   AUTH_USERNAME - Dashboard username (optional, for fetching JWT)
#   AUTH_PASSWORD - Dashboard password (optional, for fetching JWT)
#
# Example (production with auth fetch):
#   export CRAWLER_URL="https://northcloud.biz/api/crawler"
#   export AUTH_URL="https://northcloud.biz/api/v1/auth"
#   export AUTH_USERNAME="your-user"
#   export AUTH_PASSWORD="your-pass"
#   ./scripts/sync-enabled-sources-jobs.sh
#
# Example (local with jq for report):
#   JWT="your-token" ./scripts/sync-enabled-sources-jobs.sh | jq .

set -e

CRAWLER_URL="${CRAWLER_URL:-http://localhost:8060}"
# When behind nginx (e.g. CRAWLER_URL=https://northcloud.biz/api/crawler), path is /admin/...; else /api/v1/admin/...
if [ "${CRAWLER_URL#*api/crawler}" != "$CRAWLER_URL" ]; then
  ENDPOINT="${CRAWLER_URL}/admin/sync-enabled-sources"
else
  ENDPOINT="${CRAWLER_URL}/api/v1/admin/sync-enabled-sources"
fi

# Fetch JWT from auth if not set but AUTH_* are provided
if [ -z "${JWT:-}" ] && [ -n "${AUTH_URL:-}" ] && [ -n "${AUTH_USERNAME:-}" ] && [ -n "${AUTH_PASSWORD:-}" ]; then
  LOGIN_RESPONSE="$(curl -s -X POST "${AUTH_URL}/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${AUTH_USERNAME}\",\"password\":\"${AUTH_PASSWORD}\"}")"
  JWT="$(echo "$LOGIN_RESPONSE" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
  if [ -z "$JWT" ]; then
    echo "Error: Failed to get JWT from auth (check AUTH_URL, AUTH_USERNAME, AUTH_PASSWORD)." >&2
    exit 1
  fi
fi

if [ -z "${JWT:-}" ]; then
  echo "Error: JWT is required. Set JWT or AUTH_URL + AUTH_USERNAME + AUTH_PASSWORD." >&2
  exit 1
fi

curl -s -X POST "${ENDPOINT}" \
  -H "Authorization: Bearer ${JWT}" \
  -H "Content-Type: application/json"

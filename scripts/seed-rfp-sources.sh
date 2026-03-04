#!/bin/bash
# Seed provincial procurement portal sources for RFP crawling.
#
# Usage:
#   ./scripts/seed-rfp-sources.sh
#
# Environment:
#   AUTH_URL            - Auth base URL (default: http://localhost:8040)
#   SOURCE_MANAGER_URL  - Source manager API (default: http://localhost:8050)
#   JWT                 - JWT token (optional; if unset, use AUTH_USERNAME + AUTH_PASSWORD to fetch)
#   AUTH_USERNAME       - Username for login (required if JWT not set)
#   AUTH_PASSWORD       - Password for login (required if JWT not set)
#   DRY_RUN             - If non-empty, only print what would be done (no API calls)

set -e

AUTH_URL="${AUTH_URL:-http://localhost:8040}"
SOURCE_MANAGER_URL="${SOURCE_MANAGER_URL:-http://localhost:8050}"

# Fetch JWT if not set
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

# Provincial procurement sources (Tier 1 — public, no login required)
SOURCES='[
  {
    "name": "BC Bid",
    "url": "https://www.bcbid.gov.bc.ca/open.dll/welcome"
  },
  {
    "name": "SaskTenders",
    "url": "https://sasktenders.ca/content/public/Search.aspx"
  },
  {
    "name": "Alberta Purchasing Connection",
    "url": "https://purchasing.alberta.ca/search"
  },
  {
    "name": "Nova Scotia Procurement",
    "url": "https://procurement.novascotia.ca/tenders"
  }
]'

count=$(echo "$SOURCES" | jq 'length')
echo "Seeding $count RFP sources..." >&2

added=0
errors=0

for i in $(seq 0 $((count - 1))); do
  name=$(echo "$SOURCES" | jq -r ".[$i].name")
  url=$(echo "$SOURCES" | jq -r ".[$i].url")

  if [ -n "${DRY_RUN:-}" ]; then
    echo "[DRY RUN] Would add: $name | $url" >&2
    ((added++)) || true
    continue
  fi

  source_payload=$(jq -n \
    --arg name "$name" \
    --arg url "$url" \
    '{
      name: $name,
      url: $url,
      rate_limit: "10",
      max_depth: 3,
      time: [],
      selectors: {},
      enabled: true,
      ingestion_mode: "spider"
    }')

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
  echo "Added: $name (id=$source_id)" >&2
  ((added++)) || true
done

echo "Done: $added added, $errors errors." >&2
[ "$errors" -eq 0 ] || exit 1

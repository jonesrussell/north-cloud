#!/usr/bin/env bash

set -euo pipefail

SOURCE_MANAGER_URL="${SOURCE_MANAGER_URL:-http://localhost:8050}"
ELASTICSEARCH_URL="${ELASTICSEARCH_URL:-http://localhost:9200}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-table}"
ELASTICSEARCH_USERNAME="${ELASTICSEARCH_USERNAME:-}"
ELASTICSEARCH_PASSWORD="${ELASTICSEARCH_PASSWORD:-}"

usage() {
  cat <<'EOF'
Audit likely orphaned URL-derived raw_content indices after source-name canonicalization.

Usage:
  scripts/audit-orphaned-raw-indices.sh [--json]

Environment:
  SOURCE_MANAGER_URL       Source Manager base URL (default: http://localhost:8050)
  ELASTICSEARCH_URL        Elasticsearch base URL (default: http://localhost:9200)
  ELASTICSEARCH_USERNAME   Optional Elasticsearch username
  ELASTICSEARCH_PASSWORD   Optional Elasticsearch password
  OUTPUT_FORMAT            table|json (default: table)

The script compares:
  - canonical raw index names derived from source-manager source names
  - legacy URL-host-derived raw index names derived from source URLs and www/non-www variants
  - actual Elasticsearch *_raw_content indices

It reports candidate legacy indices with:
  - whether the canonical replacement index exists
  - total docs
  - pending classification backlog
  - latest crawled_at timestamp
  - a recommended review state
EOF
}

for arg in "$@"; do
  case "$arg" in
    --json)
      OUTPUT_FORMAT="json"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required dependency: $1" >&2
    exit 1
  fi
}

require_bin curl
require_bin jq
require_bin column

curl_args=()
if [[ -n "$ELASTICSEARCH_USERNAME" || -n "$ELASTICSEARCH_PASSWORD" ]]; then
  curl_args+=(-u "${ELASTICSEARCH_USERNAME}:${ELASTICSEARCH_PASSWORD}")
fi

sm_endpoint() {
  local base="${SOURCE_MANAGER_URL%/}"
  if [[ "$base" == */api/v1/sources ]]; then
    printf '%s\n' "$base"
    return
  fi
  printf '%s/api/v1/sources\n' "$base"
}

es_get() {
  curl -fsS "${curl_args[@]}" "$@"
}

es_post_json() {
  local url=$1
  local body=$2
  curl -fsS "${curl_args[@]}" \
    -H 'Content-Type: application/json' \
    -X POST \
    "$url" \
    -d "$body"
}

sanitize_source_name() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//; s/[^a-z0-9_]+/_/g; s/_+/_/g; s/^_+//; s/_+$//'
}

legacy_source_base_from_host() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/\./_/g'
}

extract_hostname() {
  printf '%s' "$1" | sed -E 's#^[a-zA-Z]+://##; s#/.*$##; s/:.*$##'
}

legacy_bases_for_url() {
  local host
  host="$(extract_hostname "$1")"
  if [[ -z "$host" ]]; then
    return
  fi

  if [[ "$host" == www.* ]]; then
    printf '%s\n' "$(legacy_source_base_from_host "$host")"
    printf '%s\n' "$(legacy_source_base_from_host "${host#www.}")"
    return
  fi

  printf '%s\n' "$(legacy_source_base_from_host "$host")"
  printf '%s\n' "$(legacy_source_base_from_host "www.${host}")"
}

collect_index_metrics() {
  local index_name=$1
  local response
  response="$(
    es_post_json \
      "${ELASTICSEARCH_URL%/}/${index_name}/_search" \
      '{"size":0,"track_total_hits":true,"aggs":{"latest_crawled_at":{"max":{"field":"crawled_at"}},"pending":{"filter":{"term":{"classification_status":"pending"}}}}}'
  )"

  jq -r '[
      (.hits.total.value // .hits.total // 0),
      (.aggregations.pending.doc_count // 0),
      (.aggregations.latest_crawled_at.value_as_string // "")
    ] | @tsv' <<<"$response"
}

results_file="$(mktemp)"
trap 'rm -f "$results_file"' EXIT

mapfile -t existing_raw_indices < <(
  es_get "${ELASTICSEARCH_URL%/}/_cat/indices/*_raw_content?format=json&h=index" \
    | jq -r '.[].index' \
    | sort -u
)

declare -A existing_index_map=()
for index_name in "${existing_raw_indices[@]}"; do
  existing_index_map["$index_name"]=1
done

declare -A seen_pairs=()
declare -A legacy_owner=()

sources_json="$(curl -fsS "$(sm_endpoint)")"

while IFS=$'\t' read -r source_name source_url enabled; do
  canonical_base="$(sanitize_source_name "$source_name")"
  canonical_index=""
  canonical_exists="false"
  if [[ -n "$canonical_base" ]]; then
    canonical_index="${canonical_base}_raw_content"
    if [[ -n "${existing_index_map[$canonical_index]:-}" ]]; then
      canonical_exists="true"
    fi
  fi

  while IFS= read -r legacy_base; do
    [[ -z "$legacy_base" ]] && continue
    [[ "$legacy_base" == "$canonical_base" ]] && continue

    legacy_index="${legacy_base}_raw_content"
    [[ -z "${existing_index_map[$legacy_index]:-}" ]] && continue

    pair_key="${legacy_index}|${canonical_index}"
    if [[ -n "${seen_pairs[$pair_key]:-}" ]]; then
      continue
    fi
    seen_pairs["$pair_key"]=1

    review_state="review_missing_canonical"
    if [[ -n "$canonical_index" && "$canonical_exists" == "true" ]]; then
      review_state="delete_candidate"
    fi

    if [[ -n "${legacy_owner[$legacy_index]:-}" && "${legacy_owner[$legacy_index]}" != "$canonical_index" ]]; then
      review_state="review_collision"
    else
      legacy_owner["$legacy_index"]="$canonical_index"
    fi

    IFS=$'\t' read -r doc_count pending_count latest_crawled_at < <(collect_index_metrics "$legacy_index")

    if [[ "$review_state" == "delete_candidate" && "$pending_count" != "0" ]]; then
      review_state="wait_pending_zero"
    fi

    jq -n \
      --arg source_name "$source_name" \
      --arg source_url "$source_url" \
      --arg enabled "$enabled" \
      --arg canonical_index "$canonical_index" \
      --arg legacy_index "$legacy_index" \
      --arg canonical_exists "$canonical_exists" \
      --arg review_state "$review_state" \
      --arg latest_crawled_at "$latest_crawled_at" \
      --argjson doc_count "$doc_count" \
      --argjson pending_count "$pending_count" \
      '{
        source_name: $source_name,
        source_url: $source_url,
        enabled: ($enabled == "true"),
        canonical_index: $canonical_index,
        canonical_exists: ($canonical_exists == "true"),
        legacy_index: $legacy_index,
        doc_count: $doc_count,
        pending_count: $pending_count,
        latest_crawled_at: $latest_crawled_at,
        review_state: $review_state
      }' >>"$results_file"
  done < <(legacy_bases_for_url "$source_url" | awk '!seen[$0]++')
done < <(jq -r '.[] | [.name // "", .url // "", ((.enabled // false) | tostring)] | @tsv' <<<"$sources_json")

results_json="$(jq -s 'sort_by(.review_state, .legacy_index, .source_name)' "$results_file")"

if [[ "$OUTPUT_FORMAT" == "json" ]]; then
  jq '.' <<<"$results_json"
  exit 0
fi

if [[ "$(jq 'length' <<<"$results_json")" -eq 0 ]]; then
  echo "No likely orphaned URL-derived raw_content indices found."
  exit 0
fi

jq -r '
  ([
    "review_state",
    "legacy_index",
    "canonical_exists",
    "canonical_index",
    "doc_count",
    "pending_count",
    "latest_crawled_at",
    "source_name"
  ] | @tsv),
  (.[] | [
    .review_state,
    .legacy_index,
    (.canonical_exists | tostring),
    .canonical_index,
    (.doc_count | tostring),
    (.pending_count | tostring),
    .latest_crawled_at,
    .source_name
  ] | @tsv)
' <<<"$results_json" | column -t -s $'\t'

cat <<'EOF'

review_state meanings:
  delete_candidate      canonical raw index exists and legacy backlog is empty
  wait_pending_zero     canonical raw index exists but legacy index still has pending docs
  review_missing_canonical  legacy raw index exists but canonical replacement index was not found
  review_collision      one legacy host-derived raw index appears to map to multiple canonical sources
EOF

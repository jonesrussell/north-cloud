#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_SCRIPT="${SCRIPT_DIR}/audit-orphaned-raw-indices.sh"

extract_function() {
  local function_name=$1

  awk -v function_name="${function_name}" '
    $0 ~ "^" function_name "\\(\\) \\{" { in_function = 1 }
    in_function { print }
    in_function && $0 == "}" { exit }
  ' "${TARGET_SCRIPT}"
}

eval "$(extract_function sanitize_source_name)"
eval "$(extract_function extract_hostname)"

assert_equals() {
  local description=$1
  local expected=$2
  local actual=$3

  if [[ "${actual}" != "${expected}" ]]; then
    echo "FAIL: ${description}: expected '${expected}', got '${actual}'" >&2
    exit 1
  fi

  echo "PASS: ${description}"
}

assert_equals \
  "extract_hostname strips scheme path and port" \
  "example.com" \
  "$(extract_hostname "https://example.com:9200/path")"

assert_equals \
  "sanitize_source_name matches ES naming semantics for spaces" \
  "campbell_river_mirror" \
  "$(sanitize_source_name "Campbell River Mirror")"

assert_equals \
  "sanitize_source_name collapses mixed punctuation" \
  "a_b_c" \
  "$(sanitize_source_name "a--b..c")"

assert_equals \
  "sanitize_source_name trims non-ascii bytes the same way as Go naming" \
  "caf_monde" \
  "$(sanitize_source_name "Café Monde")"

echo "All audit-orphaned-raw-indices.sh helper checks passed."

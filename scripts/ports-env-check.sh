#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export OUT_MD="$TMP/ports-and-env.md"
export OUT_JSON="$TMP/ports-and-env.json"
scripts/generate-ports-env-table.sh --out-md "$OUT_MD" --out-json "$OUT_JSON"
diff -u docs/generated/ports-and-env.md "$OUT_MD" || {
  echo "docs/generated/ports-and-env.md is out of date. Run: task ports:generate" >&2
  exit 1
}
diff -u docs/generated/ports-and-env.json "$OUT_JSON" || {
  echo "docs/generated/ports-and-env.json is out of date. Run: task ports:generate" >&2
  exit 1
}

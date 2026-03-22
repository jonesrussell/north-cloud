#!/bin/bash
# Layer Checker: Verifies internal package imports respect layer boundaries.
# Each service with a .layers file gets checked.
#
# Usage: tools/layer-checker.sh [service...]
# Default: checks all services with .layers files
#
# .layers format:
#   LAYER_NUMBER package_suffix    (e.g., "0 domain")
#   exempt package_suffix          (e.g., "exempt bootstrap")
#   allow SOURCE TARGET            (e.g., "allow config fetcher" — known violation, tracked)
#   # comments and blank lines are ignored
#
# Rules:
#   - A package at layer N may import packages at layer <= N
#   - A package at layer N may NOT import packages at layer > N
#   - Exempt packages (bootstrap) may import anything
#   - Subpackages inherit their parent's layer unless explicitly mapped

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXIT_CODE=0
TOTAL_SERVICES=0
TOTAL_VIOLATIONS=0

# Find services to check
if [[ $# -gt 0 ]]; then
  SERVICES=("$@")
else
  SERVICES=()
  for f in "$REPO_ROOT"/*/.layers; do
    [[ -f "$f" ]] && SERVICES+=("$(basename "$(dirname "$f")")")
  done
fi

if [[ ${#SERVICES[@]} -eq 0 ]]; then
  echo "No services with .layers files found."
  exit 0
fi

for SERVICE in "${SERVICES[@]}"; do
  LAYERS_FILE="$REPO_ROOT/$SERVICE/.layers"
  if [[ ! -f "$LAYERS_FILE" ]]; then
    echo "SKIP: $SERVICE (no .layers file)"
    continue
  fi

  SERVICE_DIR="$REPO_ROOT/$SERVICE"
  MODULE=$(cd "$SERVICE_DIR" && GOWORK=off GOFLAGS=-mod=mod go list -m 2>/dev/null) || continue
  INTERNAL_PREFIX="$MODULE/internal/"
  TOTAL_SERVICES=$((TOTAL_SERVICES + 1))

  # Parse .layers file
  declare -A PKG_LAYER=()
  declare -A EXEMPT=()
  declare -A ALLOWED=()
  while IFS= read -r line; do
    line="${line%%#*}"  # Strip inline comments
    line="${line#"${line%%[![:space:]]*}"}"  # Trim leading whitespace
    line="${line%"${line##*[![:space:]]}"}"  # Trim trailing whitespace
    [[ -z "$line" ]] && continue

    read -r first second third <<< "$line"
    if [[ "$first" == "exempt" ]]; then
      EXEMPT["$second"]=1
    elif [[ "$first" == "allow" ]]; then
      ALLOWED["$second→$third"]=1
    else
      PKG_LAYER["$second"]=$first
    fi
  done < "$LAYERS_FILE"

  # Lookup: find layer for a package suffix (exact match, then walk parents)
  get_layer() {
    local suffix="$1"
    local lookup="$suffix"
    while [[ -n "$lookup" ]]; do
      if [[ -n "${PKG_LAYER[$lookup]+x}" ]]; then
        echo "${PKG_LAYER[$lookup]}"
        return
      fi
      local parent="${lookup%/*}"
      [[ "$parent" == "$lookup" ]] && break
      lookup="$parent"
    done
    echo ""
  }

  is_exempt() {
    local suffix="$1"
    local lookup="$suffix"
    while [[ -n "$lookup" ]]; do
      [[ -n "${EXEMPT[$lookup]+x}" ]] && return 0
      local parent="${lookup%/*}"
      [[ "$parent" == "$lookup" ]] && break
      lookup="$parent"
    done
    return 1
  }

  VIOLATIONS=0
  UNMAPPED=()

  while IFS= read -r line; do
    PKG_PATH="${line%% *}"
    IMPORTS="${line#* }"
    [[ "$PKG_PATH" == "$IMPORTS" ]] && IMPORTS=""

    PKG_SUFFIX="${PKG_PATH#$INTERNAL_PREFIX}"
    [[ "$PKG_SUFFIX" == "$PKG_PATH" ]] && continue

    # Skip exempt packages
    is_exempt "$PKG_SUFFIX" && continue

    MY_LAYER=$(get_layer "$PKG_SUFFIX")
    if [[ -z "$MY_LAYER" ]]; then
      UNMAPPED+=("$PKG_SUFFIX")
      continue
    fi

    for imp in $IMPORTS; do
      IMP_SUFFIX="${imp#$INTERNAL_PREFIX}"
      [[ "$IMP_SUFFIX" == "$imp" ]] && continue  # External import, skip

      IMP_LAYER=$(get_layer "$IMP_SUFFIX")
      [[ -z "$IMP_LAYER" ]] && continue

      if (( IMP_LAYER > MY_LAYER )); then
        # Check if this specific edge is allowed
        if [[ -n "${ALLOWED["$PKG_SUFFIX→$IMP_SUFFIX"]+x}" ]]; then
          continue
        fi
        # Also check parent-based allow (e.g., "allow config fetcher" matches "config → fetcher/subpkg")
        src_base="${PKG_SUFFIX%%/*}"
        imp_base="${IMP_SUFFIX%%/*}"
        if [[ -n "${ALLOWED["$src_base→$imp_base"]+x}" ]]; then
          continue
        fi
        echo "  FAIL: $SERVICE: $PKG_SUFFIX (L$MY_LAYER) → $IMP_SUFFIX (L$IMP_LAYER)"
        VIOLATIONS=$((VIOLATIONS + 1))
      fi
    done
  done < <(cd "$SERVICE_DIR" && GOWORK=off GOFLAGS=-mod=mod go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./internal/... 2>/dev/null)

  if [[ ${#UNMAPPED[@]} -gt 0 ]]; then
    # Deduplicate
    readarray -t UNIQUE < <(printf '%s\n' "${UNMAPPED[@]}" | sort -u)
    for pkg in "${UNIQUE[@]}"; do
      echo "  WARN: $SERVICE: unmapped package '$pkg' — add it to .layers"
    done
  fi

  if [[ $VIOLATIONS -eq 0 ]]; then
    echo "  OK: $SERVICE — no layer violations"
  else
    echo "  FAIL: $SERVICE — $VIOLATIONS layer violation(s)"
    TOTAL_VIOLATIONS=$((TOTAL_VIOLATIONS + VIOLATIONS))
    EXIT_CODE=1
  fi

  unset PKG_LAYER EXEMPT ALLOWED
  declare -A PKG_LAYER=()
  declare -A EXEMPT=()
  declare -A ALLOWED=()
done

echo ""
echo "Checked $TOTAL_SERVICES service(s), $TOTAL_VIOLATIONS total violation(s)."
exit $EXIT_CODE

#!/bin/bash
# Drift Detector: Maps recent file changes to affected specs and skills.
# Usage: tools/drift-detector.sh [num_commits]
# Default: checks last 5 commits

set -euo pipefail

COMMITS=${1:-5}

# Validate input
if ! [[ "$COMMITS" =~ ^[1-9][0-9]*$ ]]; then
  echo "ERROR: num_commits must be a positive integer, got '$COMMITS'" >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Mapping: file pattern -> spec file
# Only include patterns that have corresponding spec files
declare -A PATTERN_TO_SPEC=(
  ["crawler/"]="docs/specs/content-acquisition.md"
  ["classifier/"]="docs/specs/classification.md"
  ["ml-sidecars/"]="docs/specs/classification.md"
  ["publisher/"]="docs/specs/content-routing.md"
  ["search/"]="docs/specs/discovery-querying.md"
  ["index-manager/"]="docs/specs/discovery-querying.md"
  ["infrastructure/"]="docs/specs/shared-infrastructure.md"
  ["rfp-ingestor/"]="docs/specs/rfp-ingestor.md"
  ["social-publisher/"]="docs/specs/social-publisher.md"
  ["source-manager/"]="docs/specs/shared-infrastructure.md"
  ["pipeline/"]="docs/specs/content-routing.md"
  ["dashboard/"]="docs/specs/discovery-querying.md"
  ["click-tracker/"]="docs/specs/discovery-querying.md"
  ["mcp-north-cloud/"]="docs/specs/shared-infrastructure.md"
)

# Get changed files in last N commits
cd "$REPO_ROOT"

# Verify git history depth is sufficient
TOTAL_COMMITS=$(git rev-list --count HEAD 2>/dev/null || echo 0)
if [ "$TOTAL_COMMITS" -eq 0 ]; then
  echo "ERROR: No git history found. Is this a git repository?" >&2
  exit 1
fi

if [ "$TOTAL_COMMITS" -lt "$COMMITS" ]; then
  echo "WARNING: Only $TOTAL_COMMITS commits available (requested $COMMITS). Checking all." >&2
  COMMITS=$((TOTAL_COMMITS - 1))
  if [ "$COMMITS" -le 0 ]; then
    # Single commit — compare against empty tree
    CHANGED_FILES=$(git diff-tree --no-commit-id --name-only -r HEAD)
  else
    CHANGED_FILES=$(git diff --name-only HEAD~"$COMMITS"..HEAD)
  fi
else
  CHANGED_FILES=$(git diff --name-only HEAD~"$COMMITS"..HEAD)
fi

if [ -z "$CHANGED_FILES" ]; then
  echo "No changes detected in the last $COMMITS commits."
  exit 0
fi

echo "=== Drift Detector ==="
echo "Checking last $COMMITS commits for spec drift..."
echo ""

declare -A AFFECTED_SPECS=()
declare -A SPEC_CHANGES=()

while IFS= read -r file; do
  [ -z "$file" ] && continue
  for pattern in "${!PATTERN_TO_SPEC[@]}"; do
    if [[ "$file" == "$pattern"* ]]; then
      spec="${PATTERN_TO_SPEC[$pattern]}"
      AFFECTED_SPECS["$spec"]=1
      SPEC_CHANGES["$spec"]="${SPEC_CHANGES[$spec]:-}  $file\n"
    fi
  done
done <<< "$CHANGED_FILES"

if [ "${#AFFECTED_SPECS[@]}" -eq 0 ]; then
  echo "No specs affected by recent changes."
  exit 0
fi

echo "Affected specs:"
echo ""

WARNINGS=0
for spec in "${!AFFECTED_SPECS[@]}"; do
  spec_path="$REPO_ROOT/$spec"
  if [ -f "$spec_path" ]; then
    # Compare git commit timestamps: when was the spec last touched vs the service code?
    spec_last_commit=$(git log -1 --format=%ct -- "$spec" 2>/dev/null)
    spec_last_commit=${spec_last_commit:-0}
    # Find the latest commit time for any matched service file
    service_last_commit=0
    for pattern in "${!PATTERN_TO_SPEC[@]}"; do
      if [ "${PATTERN_TO_SPEC[$pattern]}" = "$spec" ]; then
        pattern_commit=$(git log -1 --format=%ct -- "$pattern" 2>/dev/null)
        pattern_commit=${pattern_commit:-0}
        if [ "$pattern_commit" -gt "$service_last_commit" ]; then
          service_last_commit=$pattern_commit
        fi
      fi
    done

    if [ "$spec_last_commit" -lt "$service_last_commit" ]; then
      echo "  WARNING: $spec may be stale (service code updated more recently)"
      WARNINGS=$((WARNINGS + 1))
    else
      echo "  OK: $spec (updated after latest service change)"
    fi
  else
    echo "  MISSING: $spec (does not exist yet)"
    WARNINGS=$((WARNINGS + 1))
  fi

  echo -e "${SPEC_CHANGES[$spec]}" | sort -u | head -10
done

echo ""
if [ $WARNINGS -gt 0 ]; then
  echo "$WARNINGS spec(s) may need review."
  exit 1
else
  echo "All affected specs are up to date."
  exit 0
fi

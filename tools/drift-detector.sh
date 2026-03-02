#!/bin/bash
# Drift Detector: Maps recent file changes to affected specs and skills.
# Usage: tools/drift-detector.sh [num_commits]
# Default: checks last 5 commits

set -euo pipefail

COMMITS=${1:-5}
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Mapping: file pattern -> spec file
declare -A PATTERN_TO_SPEC=(
  ["crawler/"]="docs/specs/content-acquisition.md"
  ["classifier/"]="docs/specs/classification.md"
  ["ml-sidecars/"]="docs/specs/classification.md"
  ["publisher/"]="docs/specs/content-routing.md"
  ["search/"]="docs/specs/discovery-querying.md"
  ["index-manager/"]="docs/specs/discovery-querying.md"
  ["infrastructure/"]="docs/specs/shared-infrastructure.md"
  ["source-manager/"]="docs/specs/source-management.md"
  ["dashboard/"]="docs/specs/dashboard.md"
  ["pipeline/"]="docs/specs/pipeline-events.md"
  ["social-publisher/"]="docs/specs/social-publishing.md"
)

# Get changed files in last N commits
cd "$REPO_ROOT"
CHANGED_FILES=$(git diff --name-only HEAD~"$COMMITS"..HEAD 2>/dev/null || git diff --name-only HEAD 2>/dev/null || echo "")

if [ -z "$CHANGED_FILES" ]; then
  echo "No changes detected in the last $COMMITS commits."
  exit 0
fi

echo "=== Drift Detector ==="
echo "Checking last $COMMITS commits for spec drift..."
echo ""

declare -A AFFECTED_SPECS
declare -A SPEC_CHANGES

for file in $CHANGED_FILES; do
  for pattern in "${!PATTERN_TO_SPEC[@]}"; do
    if [[ "$file" == "$pattern"* ]]; then
      spec="${PATTERN_TO_SPEC[$pattern]}"
      AFFECTED_SPECS["$spec"]=1
      SPEC_CHANGES["$spec"]="${SPEC_CHANGES[$spec]:-}  $file\n"
    fi
  done
done

if [ ${#AFFECTED_SPECS[@]} -eq 0 ]; then
  echo "No specs affected by recent changes."
  exit 0
fi

echo "Affected specs:"
echo ""

WARNINGS=0
for spec in "${!AFFECTED_SPECS[@]}"; do
  spec_path="$REPO_ROOT/$spec"
  if [ -f "$spec_path" ]; then
    # Check if spec was updated more recently than the code changes
    spec_mtime=$(stat -c %Y "$spec_path" 2>/dev/null || stat -f %m "$spec_path" 2>/dev/null || echo 0)
    latest_commit_time=$(git log -1 --format=%ct HEAD 2>/dev/null || echo 0)

    if [ "$spec_mtime" -lt "$latest_commit_time" ]; then
      echo "  WARNING: $spec may be stale"
      WARNINGS=$((WARNINGS + 1))
    else
      echo "  OK: $spec (recently updated)"
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

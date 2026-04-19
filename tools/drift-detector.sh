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
  ["signal-crawler/"]="docs/specs/lead-pipeline.md"
  ["social-publisher/"]="docs/specs/social-publisher.md"
  ["source-manager/"]="docs/specs/source-manager.md"
  ["pipeline/"]="docs/specs/pipeline.md"
  ["dashboard/"]="docs/specs/dashboard.md"
  ["click-tracker/"]="docs/specs/click-tracker.md"
  ["mcp-north-cloud/"]="docs/specs/mcp-server.md"
  ["ai-observer/"]="docs/specs/ai-observer.md"
  ["auth/"]="docs/specs/auth.md"
  ["nc-http-proxy/"]="docs/specs/nc-http-proxy.md"
  ["search-frontend/"]="docs/specs/search-frontend.md"
  ["render-worker/"]="docs/specs/content-acquisition.md"
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

# Exclude vendor directories (tracked by .gitignore, but deletions still show in diff)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v '/vendor/' || true)

# Exclude files that don't affect spec accuracy (#516)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v '\.layers$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v 'CLAUDE\.md$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v 'MIGRATION\.md$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v '_test\.go$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v 'go\.mod$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v 'go\.sum$' || true)
CHANGED_FILES=$(echo "$CHANGED_FILES" | grep -v '/Dockerfile$' || true)

if [ -z "$CHANGED_FILES" ]; then
  echo "No changes detected in the last $COMMITS commits."
  exit 0
fi

echo "=== Drift Detector ==="
echo "Checking last $COMMITS commits for spec drift..."
echo ""

declare -A AFFECTED_SPECS=()
declare -A SPEC_CHANGES=()

record_spec() {
  local spec="$1" file="$2"
  AFFECTED_SPECS["$spec"]=1
  SPEC_CHANGES["$spec"]="${SPEC_CHANGES[$spec]:-}  $file\n"
}

while IFS= read -r file; do
  [ -z "$file" ] && continue

  for pattern in "${!PATTERN_TO_SPEC[@]}"; do
    if [[ "$file" == "$pattern"* ]]; then
      record_spec "${PATTERN_TO_SPEC[$pattern]}" "$file"
    fi
  done

  # Secondary mappings: files that affect additional specs beyond primary pattern
  case "$file" in
    */elasticsearch/mappings/*|*/elasticsearch/mapping*)
      record_spec "docs/specs/discovery-querying.md" "$file" ;;
    infrastructure/config/*|infrastructure/logger/*)
      record_spec "docs/specs/shared-infrastructure.md" "$file" ;;
    */internal/api/middleware*|infrastructure/middleware/*)
      record_spec "docs/specs/shared-infrastructure.md" "$file" ;;
    publisher/internal/router/*|publisher/internal/routing/*)
      record_spec "docs/specs/content-routing.md" "$file" ;;
    classifier/internal/classifier/need_signal*|classifier/internal/classifier/content_type_need_signal_heuristic*)
      record_spec "docs/specs/lead-pipeline.md" "$file" ;;
    publisher/internal/api/leads_export_handler*|publisher/internal/models/claudriel_lead*|publisher/internal/database/claudriel_lead*)
      record_spec "docs/specs/lead-pipeline.md" "$file" ;;
    infrastructure/signal/*)
      record_spec "docs/specs/lead-pipeline.md" "$file" ;;
    */migrations/*)
      ;; # migrations are already covered by primary patterns
  esac
done <<< "$CHANGED_FILES"

if [ "${#AFFECTED_SPECS[@]}" -eq 0 ]; then
  echo "No specs affected by recent changes."
  exit 0
fi

echo "Affected specs:"
echo ""

STALE_COUNT=0
for spec in $(printf '%s\n' "${!AFFECTED_SPECS[@]}" | sort); do
  spec_path="$REPO_ROOT/$spec"
  if [ -f "$spec_path" ]; then
    # Compare git commit timestamps: when was the spec last touched vs the service code?
    spec_last_commit=$(git log -1 --format=%ct -- "$spec" 2>/dev/null)
    spec_last_commit=${spec_last_commit:-0}
    # Find the latest commit time for any matched service file
    service_last_commit=0
    for pattern in "${!PATTERN_TO_SPEC[@]}"; do
      if [ "${PATTERN_TO_SPEC[$pattern]}" = "$spec" ]; then
        pattern_commit=$(git log -1 --format=%ct -- "$pattern" ':!*/vendor/*' 2>/dev/null)
        pattern_commit=${pattern_commit:-0}
        if [ "$pattern_commit" -gt "$service_last_commit" ]; then
          service_last_commit=$pattern_commit
        fi
      fi
    done

    if [ "$spec_last_commit" -lt "$service_last_commit" ]; then
      echo "  STALE: $spec"
      echo "    Fix: Review and update this spec to reflect recent service changes"
      echo "    Changed files:"
      spec_commit_hash=$(git log -1 --format=%H -- "$spec" 2>/dev/null)
      if [ -n "$spec_commit_hash" ]; then
        for pattern in "${!PATTERN_TO_SPEC[@]}"; do
          if [ "${PATTERN_TO_SPEC[$pattern]}" = "$spec" ]; then
            git diff --name-only "$spec_commit_hash"..HEAD -- "$pattern" 2>/dev/null | grep -v '/vendor/' | while read -r changed; do
              echo "      - $changed"
            done
          fi
        done
      fi
      STALE_COUNT=$((STALE_COUNT + 1))
    else
      echo "  OK: $spec"
    fi
  else
    echo "  MISSING: $spec"
    echo "    Fix: Create this spec file to document the service"
    STALE_COUNT=$((STALE_COUNT + 1))
  fi

  echo "    Changed files:"
  echo -e "${SPEC_CHANGES[$spec]}" | sort -u | grep -v '^[[:space:]]*$' | head -10 | sed 's/^/      /'
done

echo ""
if [ $STALE_COUNT -gt 0 ]; then
  echo "$STALE_COUNT spec(s) need review. Update specs before merging."
  exit 1
else
  echo "All affected specs are up to date."
  exit 0
fi

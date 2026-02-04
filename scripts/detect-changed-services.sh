#!/usr/bin/env bash
# Detect which services have changed between commits
# Usage: ./scripts/detect-changed-services.sh [base_ref]
# Output: JSON array of changed service names, or "all" if infrastructure changed

set -euo pipefail

# Service directories (must match Taskfile.yml canonical list)
GO_SERVICES=(auth classifier crawler index-manager mcp-north-cloud publisher search source-manager nc-http-proxy)
FRONTEND_SERVICES=(dashboard search-frontend)
ALL_SERVICES=("${GO_SERVICES[@]}" "${FRONTEND_SERVICES[@]}")

# Files that should trigger all services to be tested
INFRASTRUCTURE_PATTERNS=(
    "^\.github/"
    "^Taskfile\.yml$"
    "^go\.work"
    "^infrastructure/"
    "^docker-compose"
    "^\.env"
)

# Get base ref (default: origin/main for PRs, HEAD~1 for pushes)
BASE_REF="${1:-}"
if [ -z "$BASE_REF" ]; then
    if [ -n "${GITHUB_BASE_REF:-}" ]; then
        BASE_REF="origin/$GITHUB_BASE_REF"
    else
        BASE_REF="HEAD~1"
    fi
fi

# Get list of changed files
CHANGED_FILES=$(git diff --name-only "$BASE_REF" HEAD 2>/dev/null || git diff --name-only HEAD~1 HEAD 2>/dev/null || echo "")

if [ -z "$CHANGED_FILES" ]; then
    echo "[]"
    exit 0
fi

# Check if infrastructure files changed (triggers all services)
for pattern in "${INFRASTRUCTURE_PATTERNS[@]}"; do
    if echo "$CHANGED_FILES" | grep -qE "$pattern"; then
        # Output all services as JSON array
        printf '["all"]\n'
        exit 0
    fi
done

# Detect which services changed
declare -A CHANGED_SERVICES

for service in "${ALL_SERVICES[@]}"; do
    # Check if any file in the service directory changed
    if echo "$CHANGED_FILES" | grep -q "^${service}/"; then
        CHANGED_SERVICES[$service]=1
    fi
done

# Also check for shared infrastructure module changes
if echo "$CHANGED_FILES" | grep -q "^infrastructure/"; then
    # Infrastructure changes affect all Go services
    for service in "${GO_SERVICES[@]}"; do
        CHANGED_SERVICES[$service]=1
    done
fi

# Output as JSON array
if [ ${#CHANGED_SERVICES[@]} -eq 0 ]; then
    echo "[]"
else
    # Build JSON array
    JSON="["
    first=true
    for service in "${!CHANGED_SERVICES[@]}"; do
        if [ "$first" = true ]; then
            first=false
        else
            JSON+=","
        fi
        JSON+="\"$service\""
    done
    JSON+="]"
    echo "$JSON"
fi

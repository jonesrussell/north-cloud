#!/usr/bin/env bash
# Detect which services have changed between commits
#
# Usage:
#   ./scripts/detect-changed-services.sh [base_ref] [--format=FORMAT]
#
# Formats:
#   json      - JSON array of service names (default for CI test)
#   deploy    - Deploy format with matrix JSON, service list, and flags
#   simple    - Space-separated list of services
#
# Environment variables:
#   FORCE_ALL=true     - Force all services (for deploy --force)
#   MANUAL_SERVICES=x  - Comma-separated override list
#
# Output (json format):
#   ["service1","service2"] or ["all"] if infrastructure changed
#
# Output (deploy format):
#   services_matrix={...}
#   services_list=service1,service2
#   has_services=true/false
#   has_infrastructure=true/false
#   has_migrations=true/false

set -euo pipefail

# Parse arguments
BASE_REF=""
FORMAT="json"
for arg in "$@"; do
    case "$arg" in
        --format=*) FORMAT="${arg#*=}" ;;
        -*) echo "Unknown option: $arg" >&2; exit 1 ;;
        *) BASE_REF="$arg" ;;
    esac
done

# Service directories (canonical list)
# Note: 'search' dir maps to 'search-service' container in deploy
GO_SERVICES=(auth classifier crawler index-manager mcp-north-cloud publisher search source-manager nc-http-proxy)
FRONTEND_SERVICES=(dashboard search-frontend)
OTHER_SERVICES=(crime-ml mining-ml coforge-ml entertainment-ml)
ALL_SERVICES=("${GO_SERVICES[@]}" "${FRONTEND_SERVICES[@]}" "${OTHER_SERVICES[@]}")

# Deploy service names (may differ from directory names)
declare -A SERVICE_TO_DEPLOY_NAME=(
    [search]="search-service"
)

# Files that should trigger all services
INFRASTRUCTURE_PATTERNS=(
    "^\.github/"
    "^Taskfile\.yml$"
    "^go\.work"
    "^docker-compose"
    "^\.env"
)

# Handle manual override (comma-separated -> space-separated)
if [ -n "${MANUAL_SERVICES:-}" ]; then
    CHANGED_SERVICES_STR=$(echo "$MANUAL_SERVICES" | tr ',' ' ')
    INFRA_CHANGED="false"
    MIGRATIONS_CHANGED="false"
# Handle force all
elif [ "${FORCE_ALL:-}" = "true" ]; then
    CHANGED_SERVICES_STR="${ALL_SERVICES[*]}"
    INFRA_CHANGED="false"
    MIGRATIONS_CHANGED="false"
else
    # Get base ref
    if [ -z "$BASE_REF" ]; then
        if [ -n "${GITHUB_BASE_REF:-}" ]; then
            BASE_REF="origin/$GITHUB_BASE_REF"
        else
            BASE_REF="HEAD~1"
        fi
    fi

    # Validate BASE_REF
    if ! git cat-file -e "$BASE_REF" 2>/dev/null; then
        BASE_REF="HEAD~1"
    fi

    # Get list of changed files
    CHANGED_FILES=$(git diff --name-only "$BASE_REF" HEAD 2>/dev/null || git diff --name-only HEAD~1 HEAD 2>/dev/null || echo "")

    if [ -z "$CHANGED_FILES" ]; then
        # No changes detected
        case "$FORMAT" in
            deploy)
                echo 'services_matrix={"service":[]}'
                echo "services_list="
                echo "has_services=false"
                echo "has_infrastructure=false"
                echo "has_migrations=false"
                ;;
            simple) echo "" ;;
            *) echo "[]" ;;
        esac
        exit 0
    fi

    # Check infrastructure changes
    INFRA_CHANGED="false"
    for pattern in "${INFRASTRUCTURE_PATTERNS[@]}"; do
        if echo "$CHANGED_FILES" | grep -qE "$pattern"; then
            INFRA_CHANGED="true"
            break
        fi
    done
    # Also check infrastructure/ directory
    if echo "$CHANGED_FILES" | grep -q "^infrastructure/"; then
        INFRA_CHANGED="true"
    fi

    # Check migration changes
    MIGRATIONS_CHANGED="false"
    if echo "$CHANGED_FILES" | grep -q "/migrations/"; then
        MIGRATIONS_CHANGED="true"
    fi

    # For test workflow, infrastructure change = run all
    if [ "$FORMAT" = "json" ] && [ "$INFRA_CHANGED" = "true" ]; then
        echo '["all"]'
        exit 0
    fi

    # Detect which services changed
    declare -A CHANGED_SERVICES_MAP
    for service in "${ALL_SERVICES[@]}"; do
        if echo "$CHANGED_FILES" | grep -q "^${service}/"; then
            CHANGED_SERVICES_MAP[$service]=1
        fi
    done

    # Infrastructure dir changes affect all Go services (shared code)
    if echo "$CHANGED_FILES" | grep -q "^infrastructure/"; then
        for service in "${GO_SERVICES[@]}"; do
            CHANGED_SERVICES_MAP[$service]=1
        done
    fi

    # index-manager/pkg/contracts/ changes affect services with contract tests
    if echo "$CHANGED_FILES" | grep -q "^index-manager/pkg/contracts/"; then
        for service in classifier publisher crawler search; do
            CHANGED_SERVICES_MAP[$service]=1
        done
    fi

    # Build space-separated list
    CHANGED_SERVICES_STR=""
    for service in "${!CHANGED_SERVICES_MAP[@]}"; do
        CHANGED_SERVICES_STR="$CHANGED_SERVICES_STR $service"
    done
    CHANGED_SERVICES_STR=$(echo "$CHANGED_SERVICES_STR" | xargs)
fi

# Output based on format
case "$FORMAT" in
    deploy)
        if [ -n "$CHANGED_SERVICES_STR" ]; then
            # Build matrix JSON for GitHub Actions
            MATRIX_JSON="["
            FIRST=true
            for svc in $CHANGED_SERVICES_STR; do
                [ "$FIRST" = true ] && FIRST=false || MATRIX_JSON="$MATRIX_JSON,"

                # Map to deploy name if different
                DEPLOY_NAME="${SERVICE_TO_DEPLOY_NAME[$svc]:-$svc}"

                # Determine context and dockerfile
                case "$svc" in
                    search)
                        CONTEXT="."
                        DOCKERFILE="./search/Dockerfile"
                        ;;
                    search-frontend|dashboard|nc-http-proxy|crime-ml|mining-ml|coforge-ml|entertainment-ml)
                        CONTEXT="./${svc}"
                        DOCKERFILE="./${svc}/Dockerfile"
                        ;;
                    *)
                        CONTEXT="."
                        DOCKERFILE="./${svc}/Dockerfile"
                        ;;
                esac

                MATRIX_JSON="$MATRIX_JSON{\"name\":\"$DEPLOY_NAME\",\"context\":\"$CONTEXT\",\"dockerfile\":\"$DOCKERFILE\"}"
            done
            MATRIX_JSON="$MATRIX_JSON]"

            echo "services_matrix={\"service\":$MATRIX_JSON}"
            # Map directory names to deploy names for docker compose
            DEPLOY_LIST=""
            for svc in $CHANGED_SERVICES_STR; do
                DEPLOY_NAME="${SERVICE_TO_DEPLOY_NAME[$svc]:-$svc}"
                DEPLOY_LIST="${DEPLOY_LIST:+$DEPLOY_LIST,}$DEPLOY_NAME"
            done
            echo "services_list=$DEPLOY_LIST"
            echo "has_services=true"
        else
            echo 'services_matrix={"service":[]}'
            echo "services_list="
            echo "has_services=false"
        fi
        echo "has_infrastructure=$INFRA_CHANGED"
        echo "has_migrations=$MIGRATIONS_CHANGED"
        ;;

    simple)
        echo "$CHANGED_SERVICES_STR"
        ;;

    json|*)
        if [ -z "$CHANGED_SERVICES_STR" ]; then
            echo "[]"
        else
            JSON="["
            FIRST=true
            for service in $CHANGED_SERVICES_STR; do
                [ "$FIRST" = true ] && FIRST=false || JSON="$JSON,"
                JSON="$JSON\"$service\""
            done
            JSON="$JSON]"
            echo "$JSON"
        fi
        ;;
esac

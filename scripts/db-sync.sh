#!/bin/bash
# Database sync script - Pull production data to development (ONE-WAY ONLY)
# Usage: ./scripts/db-sync.sh <service|all> [--dry-run]
#
# Arguments:
#   service   - Service name: crawler, source-manager, classifier, publisher, index-manager
#               Or 'all' to sync all databases
#   --dry-run - Preview what would happen without executing
#
# Environment variables (required):
#   PROD_SSH_HOST     - SSH connection string (e.g., user@production-server)
#   PROD_SSH_PORT     - SSH port (default: 22)
#   PROD_DEPLOY_PATH  - Path to north-cloud on production server (default: /opt/north-cloud)
#
# Safety features:
#   - Refuses to run if APP_ENV=production (cannot sync TO production)
#   - One-way only (production → development)
#   - SSH-based (uses existing deployment credentials)
#
# Examples:
#   ./scripts/db-sync.sh crawler               # Sync crawler database from prod
#   ./scripts/db-sync.sh all                   # Sync all databases from prod
#   ./scripts/db-sync.sh crawler --dry-run    # Preview without executing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/db-utils.sh
source "${SCRIPT_DIR}/db-utils.sh"

# =============================================================================
# Configuration
# =============================================================================

SERVICE="$1"
DRY_RUN=false

# Parse arguments
shift || true
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

PROD_SSH_HOST="${PROD_SSH_HOST:-}"
PROD_SSH_PORT="${PROD_SSH_PORT:-22}"
PROD_DEPLOY_PATH="${PROD_DEPLOY_PATH:-/opt/north-cloud}"

if [ -z "$SERVICE" ]; then
    echo "Usage: $0 <service|all> [--dry-run]"
    echo ""
    echo "Services: $ALL_SERVICES"
    echo "          all - sync all databases"
    echo ""
    echo "Options:"
    echo "  --dry-run  Preview what would happen without executing"
    echo ""
    echo "Required environment variables:"
    echo "  PROD_SSH_HOST     SSH connection string (e.g., user@production-server)"
    echo "  PROD_SSH_PORT     SSH port (default: 22)"
    echo "  PROD_DEPLOY_PATH  Path to north-cloud on production (default: /opt/north-cloud)"
    echo ""
    echo "Examples:"
    echo "  $0 crawler"
    echo "  $0 all --dry-run"
    exit 1
fi

# =============================================================================
# Safety Checks
# =============================================================================

# CRITICAL: Refuse to run in production
if is_production; then
    log_error "SAFETY: Cannot run db-sync in production environment!"
    log_error "This script is designed to PULL data FROM production TO development."
    log_error "Running it in production would overwrite production data."
    exit 1
fi

# Check SSH configuration
if [ -z "$PROD_SSH_HOST" ]; then
    log_error "PROD_SSH_HOST environment variable is not set"
    log_error "Set it to your production SSH connection string (e.g., user@production-server)"
    exit 1
fi

# =============================================================================
# Sync Functions
# =============================================================================

# Test SSH connection
test_ssh_connection() {
    log_info "Testing SSH connection to $PROD_SSH_HOST..."
    if ssh -p "$PROD_SSH_PORT" -o ConnectTimeout=10 -o BatchMode=yes "$PROD_SSH_HOST" "echo 'Connection successful'" 2>/dev/null; then
        log_success "SSH connection verified"
        return 0
    else
        log_error "Cannot connect to $PROD_SSH_HOST"
        log_error "Make sure you have SSH key access configured"
        return 1
    fi
}

# Sync single service
# Usage: sync_service <service>
sync_service() {
    local service="$1"
    local timestamp
    timestamp=$(date +%Y%m%d_%H%M%S)
    local remote_backup_name="${service}_prod_${timestamp}.sql.gz"
    local remote_backup_path="/tmp/${remote_backup_name}"
    local local_backup_path

    log_info "Syncing $service database from production..."

    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] Would execute the following steps:"
        echo "  1. SSH to $PROD_SSH_HOST"
        echo "  2. Run db-backup.sh on production for $service"
        echo "  3. SCP backup to local machine"
        echo "  4. Restore to local $service database"
        echo "  5. Cleanup remote temp file"
        return 0
    fi

    # Ensure local backup directory exists
    local_backup_path=$(ensure_backup_dir "$service")/${remote_backup_name}

    # Step 1: Create backup on production
    log_info "Creating backup on production server..."
    local remote_result
    remote_result=$(ssh -p "$PROD_SSH_PORT" "$PROD_SSH_HOST" \
        "cd ${PROD_DEPLOY_PATH} && ./scripts/db-backup.sh ${service} 2>&1 | tail -1")

    if [ -z "$remote_result" ] || [ ! -n "$remote_result" ]; then
        log_error "Failed to create backup on production"
        return 1
    fi

    # The backup script outputs the file path as the last line
    local remote_backup_file="$remote_result"
    log_info "Production backup created: $remote_backup_file"

    # Step 2: Copy backup to local
    log_info "Downloading backup from production..."
    if ! scp -P "$PROD_SSH_PORT" "${PROD_SSH_HOST}:${remote_backup_file}" "$local_backup_path"; then
        log_error "Failed to download backup from production"
        return 1
    fi
    log_success "Downloaded: $local_backup_path"

    # Step 3: Restore locally
    log_info "Restoring to local database..."
    if ! "${SCRIPT_DIR}/db-restore.sh" "$service" "$local_backup_path" --confirm; then
        log_error "Failed to restore database locally"
        return 1
    fi

    # Step 4: Optionally cleanup remote backup (keep local copy for safety)
    log_info "Cleaning up remote temp file..."
    ssh -p "$PROD_SSH_PORT" "$PROD_SSH_HOST" "rm -f '$remote_backup_file'" 2>/dev/null || true

    log_success "Sync completed for $service"
    log_info "Local backup retained at: $local_backup_path"

    return 0
}

# =============================================================================
# Main
# =============================================================================

log_info "Database Sync: Production → Development"
log_info "Production host: $PROD_SSH_HOST"
log_info "Production path: $PROD_DEPLOY_PATH"

if [ "$DRY_RUN" = true ]; then
    log_warning "DRY-RUN MODE: No changes will be made"
fi

echo ""

# Test SSH connection first
if [ "$DRY_RUN" = false ]; then
    if ! test_ssh_connection; then
        exit 1
    fi
    echo ""
fi

if [ "$SERVICE" = "all" ]; then
    log_info "Syncing all databases from production..."
    echo ""

    failed=0
    for svc in $ALL_SERVICES; do
        if sync_service "$svc"; then
            echo ""
        else
            failed=$((failed + 1))
            echo ""
        fi
    done

    if [ $failed -gt 0 ]; then
        log_error "$failed sync(s) failed"
        exit 1
    fi

    log_success "All databases synced successfully!"
else
    if ! validate_service "$SERVICE"; then
        log_error "Unknown service: $SERVICE"
        echo "Valid services: $ALL_SERVICES"
        exit 1
    fi

    sync_service "$SERVICE"
fi

#!/bin/bash
# Database backup script for North Cloud services
# Usage: ./scripts/db-backup.sh <service|all> [--retention N]
#
# Arguments:
#   service   - Service name: crawler, source-manager, classifier, publisher, index-manager
#               Or 'all' to backup all databases
#   --retention N - Keep N most recent backups per service (default: 7)
#
# Environment variables:
#   DB_BACKUP_DIR       - Backup directory (default: ./backups)
#   DB_BACKUP_RETENTION - Default retention count (default: 7)
#
# Examples:
#   ./scripts/db-backup.sh crawler              # Backup crawler database
#   ./scripts/db-backup.sh all                  # Backup all databases
#   ./scripts/db-backup.sh all --retention 14   # Keep 14 backups per service

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/db-utils.sh
source "${SCRIPT_DIR}/db-utils.sh"

# =============================================================================
# Configuration
# =============================================================================

SERVICE="$1"
RETENTION="${DB_BACKUP_RETENTION:-7}"

# Parse arguments
shift || true
while [[ $# -gt 0 ]]; do
    case "$1" in
        --retention)
            RETENTION="$2"
            shift 2
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [ -z "$SERVICE" ]; then
    echo "Usage: $0 <service|all> [--retention N]"
    echo ""
    echo "Services: $ALL_SERVICES"
    echo "          all - backup all databases"
    echo ""
    echo "Options:"
    echo "  --retention N  Keep N most recent backups per service (default: $RETENTION)"
    echo ""
    echo "Examples:"
    echo "  $0 crawler"
    echo "  $0 all --retention 14"
    exit 1
fi

# =============================================================================
# Backup Functions
# =============================================================================

# Run pg_dump via docker
# Usage: run_pg_dump <output_file>
run_pg_dump() {
    local output_file="$1"
    local db_url="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOSTNAME}:${DB_INTERNAL_PORT}/${DB_NAME}?sslmode=disable"

    if [ "$ENV_MODE" = "localhost" ]; then
        # Direct connection
        PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOSTNAME" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" | gzip > "$output_file"
    else
        # Run pg_dump via docker
        docker run --rm \
            --network "$NETWORK" \
            postgres:16-alpine \
            pg_dump "$db_url" | gzip > "$output_file"
    fi
}

# Apply retention policy
# Usage: apply_retention <service>
apply_retention() {
    local service="$1"
    local backup_dir
    backup_dir=$(get_backup_dir "$service")

    # Count existing backups
    local count
    count=$(find "$backup_dir" -name "*.sql.gz" -type f 2>/dev/null | wc -l)

    if [ "$count" -gt "$RETENTION" ]; then
        local to_delete=$((count - RETENTION))
        log_info "Applying retention policy: removing $to_delete old backup(s)"

        # Delete oldest backups
        find "$backup_dir" -name "*.sql.gz" -type f -printf '%T+ %p\n' | \
            sort | \
            head -n "$to_delete" | \
            cut -d' ' -f2- | \
            xargs rm -f
    fi
}

# Backup single service
# Usage: backup_service <service>
backup_service() {
    local service="$1"

    log_info "Backing up $service database..."

    # Resolve connection variables
    if ! resolve_db_vars "$service"; then
        log_error "Failed to resolve database configuration for $service"
        return 1
    fi

    log_info "Environment: $ENV_MODE, Database: $DB_NAME"

    # Ensure backup directory exists
    local backup_dir
    backup_dir=$(ensure_backup_dir "$service")

    # Generate filename
    local filename
    filename=$(generate_backup_filename "$service")
    local backup_path="${backup_dir}/${filename}"

    # Run backup
    log_info "Creating backup: $backup_path"
    if run_pg_dump "$backup_path"; then
        local size
        size=$(du -h "$backup_path" | cut -f1)
        log_success "Backup completed: $backup_path ($size)"

        # Apply retention policy
        apply_retention "$service"

        echo "$backup_path"
        return 0
    else
        log_error "Backup failed for $service"
        rm -f "$backup_path"
        return 1
    fi
}

# =============================================================================
# Main
# =============================================================================

if [ "$SERVICE" = "all" ]; then
    log_info "Backing up all databases (retention: $RETENTION)"
    echo ""

    failed=0
    for svc in $ALL_SERVICES; do
        if backup_service "$svc"; then
            echo ""
        else
            failed=$((failed + 1))
            echo ""
        fi
    done

    if [ $failed -gt 0 ]; then
        log_error "$failed backup(s) failed"
        exit 1
    fi

    log_success "All backups completed successfully"
else
    if ! validate_service "$SERVICE"; then
        log_error "Unknown service: $SERVICE"
        echo "Valid services: $ALL_SERVICES"
        exit 1
    fi

    backup_service "$SERVICE"
fi

#!/bin/bash
# List available database backups
# Usage: ./scripts/db-list.sh [service]
#
# Arguments:
#   service - (Optional) Service name to filter by
#
# Examples:
#   ./scripts/db-list.sh           # List all backups
#   ./scripts/db-list.sh crawler   # List only crawler backups

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/db-utils.sh
source "${SCRIPT_DIR}/db-utils.sh"

# =============================================================================
# Configuration
# =============================================================================

SERVICE="${1:-}"
BACKUP_DIR="${DB_BACKUP_DIR:-./backups}"

# =============================================================================
# List Functions
# =============================================================================

list_service_backups() {
    local service="$1"
    local service_dir="${BACKUP_DIR}/${service}"

    if [ ! -d "$service_dir" ]; then
        return 0
    fi

    local backups
    backups=$(find "$service_dir" -name "*.sql.gz" -type f 2>/dev/null | sort -r)

    if [ -z "$backups" ]; then
        return 0
    fi

    echo -e "${BLUE}${service}${NC}:"

    while IFS= read -r backup; do
        local size
        local date_modified
        size=$(du -h "$backup" | cut -f1)
        date_modified=$(stat -c '%y' "$backup" 2>/dev/null | cut -d'.' -f1 || stat -f '%Sm' "$backup" 2>/dev/null)
        local filename
        filename=$(basename "$backup")
        printf "  %-50s %8s  %s\n" "$filename" "$size" "$date_modified"
    done <<< "$backups"

    echo ""
}

# =============================================================================
# Main
# =============================================================================

if [ ! -d "$BACKUP_DIR" ]; then
    log_warning "Backup directory does not exist: $BACKUP_DIR"
    log_info "No backups found. Run 'task db:backup:<service>' to create backups."
    exit 0
fi

echo ""
echo "Available backups in ${BACKUP_DIR}:"
echo ""

if [ -n "$SERVICE" ]; then
    if ! validate_service "$SERVICE"; then
        log_error "Unknown service: $SERVICE"
        echo "Valid services: $ALL_SERVICES"
        exit 1
    fi

    list_service_backups "$SERVICE"
else
    for svc in $ALL_SERVICES; do
        list_service_backups "$svc"
    done
fi

# Summary
total_count=$(find "$BACKUP_DIR" -name "*.sql.gz" -type f 2>/dev/null | wc -l)
total_size=$(du -sh "$BACKUP_DIR" 2>/dev/null | cut -f1)

if [ "$total_count" -eq 0 ]; then
    log_info "No backups found."
else
    echo "---"
    echo "Total: $total_count backup(s), $total_size"
fi

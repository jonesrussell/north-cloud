#!/bin/bash
# Database restore script for North Cloud services
# Usage: ./scripts/db-restore.sh <service> <backup_file> --confirm
#
# Arguments:
#   service     - Service name: crawler, source-manager, classifier, publisher, index-manager
#   backup_file - Path to the .sql.gz backup file
#   --confirm   - Required flag to prevent accidental restores
#
# Safety features:
#   - Requires --confirm flag (no accidental restores)
#   - Auto-backup before restore
#   - Extra confirmation in production environment
#
# Examples:
#   ./scripts/db-restore.sh crawler ./backups/crawler/backup.sql.gz --confirm

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/db-utils.sh
source "${SCRIPT_DIR}/db-utils.sh"

# =============================================================================
# Configuration
# =============================================================================

SERVICE="$1"
BACKUP_FILE="$2"
CONFIRM_FLAG="$3"

if [ -z "$SERVICE" ] || [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <service> <backup_file> --confirm"
    echo ""
    echo "Services: $ALL_SERVICES"
    echo ""
    echo "Arguments:"
    echo "  service     - Service name to restore"
    echo "  backup_file - Path to .sql.gz backup file"
    echo "  --confirm   - Required flag to prevent accidental restores"
    echo ""
    echo "Examples:"
    echo "  $0 crawler ./backups/crawler/crawler_crawler_20260126_143022.sql.gz --confirm"
    exit 1
fi

# =============================================================================
# Validation
# =============================================================================

if ! validate_service "$SERVICE"; then
    log_error "Unknown service: $SERVICE"
    echo "Valid services: $ALL_SERVICES"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    log_error "Backup file not found: $BACKUP_FILE"
    exit 1
fi

if ! require_confirmation "Restore requires confirmation to prevent data loss." "$CONFIRM_FLAG"; then
    exit 1
fi

# =============================================================================
# Restore Functions
# =============================================================================

# Run psql restore via docker
# Usage: run_restore <backup_file>
run_restore() {
    local backup_file="$1"
    local db_url="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOSTNAME}:${DB_INTERNAL_PORT}/${DB_NAME}?sslmode=disable"

    if [ "$ENV_MODE" = "localhost" ]; then
        # Direct connection
        gunzip -c "$backup_file" | PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOSTNAME" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -q
    else
        # Run psql via docker
        gunzip -c "$backup_file" | docker run --rm -i \
            --network "$NETWORK" \
            postgres:16-alpine \
            psql "$db_url" -q
    fi
}

# Drop and recreate database
# Usage: reset_database
reset_database() {
    local db_url="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOSTNAME}:${DB_INTERNAL_PORT}/postgres?sslmode=disable"

    log_info "Resetting database $DB_NAME..."

    if [ "$ENV_MODE" = "localhost" ]; then
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOSTNAME" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS \"$DB_NAME\";"
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOSTNAME" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE \"$DB_NAME\";"
    else
        docker run --rm \
            --network "$NETWORK" \
            postgres:16-alpine \
            psql "$db_url" -c "DROP DATABASE IF EXISTS \"$DB_NAME\";"

        docker run --rm \
            --network "$NETWORK" \
            postgres:16-alpine \
            psql "$db_url" -c "CREATE DATABASE \"$DB_NAME\";"
    fi
}

# =============================================================================
# Main
# =============================================================================

log_info "Preparing to restore $SERVICE database from: $BACKUP_FILE"

# Resolve connection variables
if ! resolve_db_vars "$SERVICE"; then
    log_error "Failed to resolve database configuration for $SERVICE"
    exit 1
fi

log_info "Environment: $ENV_MODE, Database: $DB_NAME"

# Production confirmation
if ! require_production_confirmation; then
    exit 1
fi

# Auto-backup before restore
log_info "Creating backup before restore..."
pre_restore_backup=$("${SCRIPT_DIR}/db-backup.sh" "$SERVICE" 2>/dev/null | tail -1)
if [ -n "$pre_restore_backup" ] && [ -f "$pre_restore_backup" ]; then
    log_success "Pre-restore backup created: $pre_restore_backup"
else
    log_warning "Could not create pre-restore backup, proceeding anyway..."
fi

# Reset and restore
log_info "Dropping and recreating database..."
if ! reset_database; then
    log_error "Failed to reset database"
    exit 1
fi

log_info "Restoring from backup..."
if run_restore "$BACKUP_FILE"; then
    log_success "Restore completed successfully!"
    log_info "Database $DB_NAME has been restored from $BACKUP_FILE"

    if [ -n "$pre_restore_backup" ] && [ -f "$pre_restore_backup" ]; then
        log_info "Previous data backed up to: $pre_restore_backup"
    fi
else
    log_error "Restore failed!"
    log_error "You may need to restore from the pre-restore backup: $pre_restore_backup"
    exit 1
fi

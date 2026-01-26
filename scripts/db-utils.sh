#!/bin/bash
# Shared utilities for database backup/restore/sync scripts
# This file should be sourced, not executed directly

# =============================================================================
# Color Output
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# =============================================================================
# Service Configuration
# =============================================================================

# Get service configuration (env prefix, default database, default port, container pattern, swarm service)
# Usage: get_service_config <service>
# Returns: Sets SERVICE_CONFIG_* variables
get_service_config() {
    local service="$1"

    case "$service" in
        crawler)
            SERVICE_CONFIG_ENV_PREFIX="POSTGRES_CRAWLER"
            SERVICE_CONFIG_DB_DEFAULT="crawler"
            SERVICE_CONFIG_PORT_DEFAULT="5433"
            SERVICE_CONFIG_CONTAINER="postgres-crawler"
            SERVICE_CONFIG_SWARM="northcloud_postgres-crawler"
            ;;
        source-manager)
            SERVICE_CONFIG_ENV_PREFIX="POSTGRES_SOURCE_MANAGER"
            SERVICE_CONFIG_DB_DEFAULT="gosources"
            SERVICE_CONFIG_PORT_DEFAULT="5432"
            SERVICE_CONFIG_CONTAINER="postgres-source-manager"
            SERVICE_CONFIG_SWARM="northcloud_postgres-source-manager"
            ;;
        classifier)
            SERVICE_CONFIG_ENV_PREFIX="POSTGRES_CLASSIFIER"
            SERVICE_CONFIG_DB_DEFAULT="classifier"
            SERVICE_CONFIG_PORT_DEFAULT="5435"
            SERVICE_CONFIG_CONTAINER="postgres-classifier"
            SERVICE_CONFIG_SWARM="northcloud_postgres-classifier"
            ;;
        publisher)
            SERVICE_CONFIG_ENV_PREFIX="POSTGRES_PUBLISHER"
            SERVICE_CONFIG_DB_DEFAULT="publisher"
            SERVICE_CONFIG_PORT_DEFAULT="5437"
            SERVICE_CONFIG_CONTAINER="postgres-publisher"
            SERVICE_CONFIG_SWARM="northcloud_postgres-publisher"
            ;;
        index-manager)
            SERVICE_CONFIG_ENV_PREFIX="POSTGRES_INDEX_MANAGER"
            SERVICE_CONFIG_DB_DEFAULT="index_manager"
            SERVICE_CONFIG_PORT_DEFAULT="5436"
            SERVICE_CONFIG_CONTAINER="postgres-index-manager"
            SERVICE_CONFIG_SWARM="northcloud_postgres-index-manager"
            ;;
        *)
            return 1
            ;;
    esac
    return 0
}

# List of all services with databases
ALL_SERVICES="crawler source-manager classifier publisher index-manager"

# Validate service name
validate_service() {
    local service="$1"
    for s in $ALL_SERVICES; do
        if [ "$s" = "$service" ]; then
            return 0
        fi
    done
    return 1
}

# =============================================================================
# Environment Detection (reused from run-migration.sh)
# =============================================================================

# Detect environment and set connection variables
# Usage: detect_environment
# Sets: ENV_MODE, NETWORK, DB_HOSTNAME, DB_INTERNAL_PORT
detect_environment() {
    local container_pattern="$1"
    local swarm_service="$2"

    # Check for Docker Swarm mode
    if docker service ls --format '{{.Name}}' 2>/dev/null | grep -q "$swarm_service"; then
        ENV_MODE="swarm"
        NETWORK="northcloud_north-cloud-network"
        DB_HOSTNAME="$swarm_service"
        DB_INTERNAL_PORT="5432"
        return
    fi

    # Check for Docker Compose mode
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "$container_pattern"; then
        ENV_MODE="compose"
        NETWORK="north-cloud_north-cloud-network"
        DB_HOSTNAME="$container_pattern"
        DB_INTERNAL_PORT="5432"
        return
    fi

    # Fallback to localhost
    ENV_MODE="localhost"
    NETWORK="host"
    DB_HOSTNAME="localhost"
    # DB_INTERNAL_PORT will be set from service config
}

# Resolve database connection variables for a service
# Usage: resolve_db_vars <service>
# Sets: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
resolve_db_vars() {
    local service="$1"

    if ! get_service_config "$service"; then
        log_error "Unknown service: $service"
        return 1
    fi

    # Build variable names dynamically
    local host_var="${SERVICE_CONFIG_ENV_PREFIX}_HOST"
    local port_var="${SERVICE_CONFIG_ENV_PREFIX}_PORT"
    local user_var="${SERVICE_CONFIG_ENV_PREFIX}_USER"
    local password_var="${SERVICE_CONFIG_ENV_PREFIX}_PASSWORD"
    local db_var="${SERVICE_CONFIG_ENV_PREFIX}_DB"

    # Get values with defaults
    DB_HOST="${!host_var:-localhost}"
    DB_PORT="${!port_var:-$SERVICE_CONFIG_PORT_DEFAULT}"
    DB_USER="${!user_var:-postgres}"
    DB_PASSWORD="${!password_var:-postgres}"
    DB_NAME="${!db_var:-$SERVICE_CONFIG_DB_DEFAULT}"

    # Detect environment and adjust connection params
    detect_environment "$SERVICE_CONFIG_CONTAINER" "$SERVICE_CONFIG_SWARM"

    if [ "$ENV_MODE" = "swarm" ]; then
        DB_PASSWORD="postgres"
        DB_NAME="$SERVICE_CONFIG_DB_DEFAULT"
    fi

    # Export password for pg_dump/psql
    export PGPASSWORD="$DB_PASSWORD"

    return 0
}

# =============================================================================
# Backup Directory Management
# =============================================================================

# Get backup directory for a service
# Usage: get_backup_dir <service>
get_backup_dir() {
    local service="$1"
    local base_dir="${DB_BACKUP_DIR:-./backups}"
    echo "${base_dir}/${service}"
}

# Ensure backup directory exists
ensure_backup_dir() {
    local service="$1"
    local dir
    dir=$(get_backup_dir "$service")
    mkdir -p "$dir"
    echo "$dir"
}

# Generate backup filename
# Usage: generate_backup_filename <service>
generate_backup_filename() {
    local service="$1"
    local timestamp
    timestamp=$(date +%Y%m%d_%H%M%S)

    if ! get_service_config "$service"; then
        return 1
    fi

    echo "${service}_${SERVICE_CONFIG_DB_DEFAULT}_${timestamp}.sql.gz"
}

# =============================================================================
# Safety Checks
# =============================================================================

# Check if running in production
is_production() {
    [ "${APP_ENV:-development}" = "production" ]
}

# Require confirmation for dangerous operations
require_confirmation() {
    local message="$1"
    local confirm_flag="$2"

    if [ "$confirm_flag" != "--confirm" ]; then
        log_error "$message"
        log_error "Add --confirm flag to proceed"
        return 1
    fi
    return 0
}

# Extra production confirmation
require_production_confirmation() {
    if is_production; then
        log_warning "Running in PRODUCTION environment!"
        echo -n "Type 'yes' to confirm: "
        read -r response
        if [ "$response" != "yes" ]; then
            log_error "Aborted"
            return 1
        fi
    fi
    return 0
}

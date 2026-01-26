#!/bin/bash
# Shared migration script for all services
# Usage: ./run-migration.sh <service> <command> [version]
#
# Arguments:
#   service  - Service name: crawler, source-manager, classifier, publisher, index-manager
#   command  - Migration command: up, down, version, force
#   version  - (Optional) Version number for 'force' command
#
# Environment variables (with defaults):
#   POSTGRES_{SERVICE}_HOST     (default: localhost)
#   POSTGRES_{SERVICE}_PORT     (default: 5432)
#   POSTGRES_{SERVICE}_USER     (default: postgres)
#   POSTGRES_{SERVICE}_PASSWORD (default: postgres)
#   POSTGRES_{SERVICE}_DB       (default: service-specific)
#
# Examples:
#   ./run-migration.sh crawler up
#   ./run-migration.sh crawler version
#   ./run-migration.sh crawler force 5
#   ./run-migration.sh publisher down

set -e

# =============================================================================
# Configuration
# =============================================================================

SERVICE="$1"
COMMAND="$2"
VERSION="$3"

if [ -z "$SERVICE" ] || [ -z "$COMMAND" ]; then
    echo "Usage: $0 <service> <command> [version]"
    echo ""
    echo "Services: crawler, source-manager, classifier, publisher, index-manager"
    echo "Commands: up, down, version, force"
    echo ""
    echo "Examples:"
    echo "  $0 crawler up"
    echo "  $0 crawler force 5"
    exit 1
fi

# Service-specific configuration
case "$SERVICE" in
    crawler)
        ENV_PREFIX="POSTGRES_CRAWLER"
        DB_DEFAULT="crawler"
        PORT_DEFAULT="5433"
        CONTAINER_PATTERN="postgres-crawler"
        SWARM_SERVICE="northcloud_postgres-crawler"
        MIGRATION_PATH="./migrations"
        ;;
    source-manager)
        ENV_PREFIX="POSTGRES_SOURCE_MANAGER"
        DB_DEFAULT="gosources"
        PORT_DEFAULT="5432"
        CONTAINER_PATTERN="postgres-source-manager"
        SWARM_SERVICE="northcloud_postgres-source-manager"
        MIGRATION_PATH="./migrations"
        ;;
    classifier)
        ENV_PREFIX="POSTGRES_CLASSIFIER"
        DB_DEFAULT="classifier"
        PORT_DEFAULT="5435"
        CONTAINER_PATTERN="postgres-classifier"
        SWARM_SERVICE="northcloud_postgres-classifier"
        MIGRATION_PATH="./migrations"
        ;;
    publisher)
        ENV_PREFIX="POSTGRES_PUBLISHER"
        DB_DEFAULT="publisher"
        PORT_DEFAULT="5432"
        CONTAINER_PATTERN="postgres-publisher"
        SWARM_SERVICE="northcloud_postgres-publisher"
        MIGRATION_PATH="./migrations"
        ;;
    index-manager)
        ENV_PREFIX="POSTGRES_INDEX_MANAGER"
        DB_DEFAULT="index_manager"
        PORT_DEFAULT="5436"
        CONTAINER_PATTERN="postgres-index-manager"
        SWARM_SERVICE="northcloud_postgres-index-manager"
        MIGRATION_PATH="./migrations"
        ;;
    *)
        echo "Unknown service: $SERVICE"
        echo "Valid services: crawler, source-manager, classifier, publisher, index-manager"
        exit 1
        ;;
esac

# =============================================================================
# Resolve environment variables
# =============================================================================

# Build variable names dynamically
HOST_VAR="${ENV_PREFIX}_HOST"
PORT_VAR="${ENV_PREFIX}_PORT"
USER_VAR="${ENV_PREFIX}_USER"
PASSWORD_VAR="${ENV_PREFIX}_PASSWORD"
DB_VAR="${ENV_PREFIX}_DB"

# Get values with defaults
DB_HOST="${!HOST_VAR:-localhost}"
DB_PORT="${!PORT_VAR:-$PORT_DEFAULT}"
DB_USER="${!USER_VAR:-postgres}"
DB_PASSWORD="${!PASSWORD_VAR:-postgres}"
DB_NAME="${!DB_VAR:-$DB_DEFAULT}"

# Export password for psql compatibility
export PGPASSWORD="$DB_PASSWORD"

# =============================================================================
# Environment Detection
# =============================================================================

detect_environment() {
    # Check for Docker Swarm mode
    if docker service ls --format '{{.Name}}' 2>/dev/null | grep -q "$SWARM_SERVICE"; then
        ENV_MODE="swarm"
        NETWORK="northcloud_north-cloud-network"
        DB_HOSTNAME="${SWARM_SERVICE}"
        DB_INTERNAL_PORT="5432"
        # Force swarm defaults
        DB_PASSWORD="postgres"
        DB_NAME="$DB_DEFAULT"
        return
    fi

    # Check for Docker Compose mode
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "$CONTAINER_PATTERN"; then
        ENV_MODE="compose"
        NETWORK="north-cloud_north-cloud-network"
        DB_HOSTNAME="${CONTAINER_PATTERN}"
        DB_INTERNAL_PORT="5432"
        return
    fi

    # Fallback to localhost
    ENV_MODE="localhost"
    NETWORK="host"
    DB_HOSTNAME="localhost"
    DB_INTERNAL_PORT="$DB_PORT"
}

# =============================================================================
# Migration Runner
# =============================================================================

run_migration() {
    local cmd="$1"
    local extra_args="$2"

    # Build database URL
    local db_url="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOSTNAME}:${DB_INTERNAL_PORT}/${DB_NAME}?sslmode=disable"

    # Build docker command based on environment
    if [ "$ENV_MODE" = "localhost" ]; then
        docker run --rm \
            -v "$(pwd)/${MIGRATION_PATH}:/migrations:ro" \
            --network host \
            migrate/migrate:latest \
            -path /migrations \
            -database "$db_url" \
            $cmd $extra_args
    else
        docker run --rm \
            -v "$(pwd)/${MIGRATION_PATH}:/migrations:ro" \
            --network "$NETWORK" \
            migrate/migrate:latest \
            -path /migrations \
            -database "$db_url" \
            $cmd $extra_args
    fi
}

# =============================================================================
# Main
# =============================================================================

# Detect environment
detect_environment

case "$ENV_MODE" in
    swarm)
        echo "Running migration via Docker Swarm (connecting to $SWARM_SERVICE service)..."
        ;;
    compose)
        echo "Running migration via Docker Compose (connecting to $CONTAINER_PATTERN container)..."
        ;;
    localhost)
        echo "Running migration via localhost connection (port $DB_PORT)..."
        ;;
esac

# Execute command
case "$COMMAND" in
    up)
        run_migration "up"
        ;;
    down)
        run_migration "down" "1"
        ;;
    version)
        run_migration "version"
        ;;
    force)
        if [ -z "$VERSION" ]; then
            echo "Error: 'force' command requires a version number"
            echo "Usage: $0 $SERVICE force <version>"
            exit 1
        fi
        echo "Forcing database to version $VERSION..."
        run_migration "force" "$VERSION"
        ;;
    *)
        echo "Unknown command: $COMMAND"
        echo "Valid commands: up, down, version, force"
        exit 1
        ;;
esac

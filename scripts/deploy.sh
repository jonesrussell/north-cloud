#!/bin/bash
# Production deployment script for North Cloud
# This script should be placed at /opt/north-cloud/deploy.sh on the production server
#
# Features:
#   - Selective service deployment (only changed services)
#   - Parallel database migrations
#   - Retry logic for transient dependency failures
#   - Automatic rollback on health check failure
#
# Environment variables (optional, set by CI):
#   CHANGED_SERVICES - Comma-separated list of services that changed (empty = all)
#   INFRA_CHANGED    - "true" if infrastructure files changed
#   MIGRATIONS_CHANGED - "true" if migration files changed

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Deployment directory
DEPLOY_DIR="/opt/north-cloud"
COMPOSE_CMD="docker compose -f docker-compose.base.yml -f docker-compose.prod.yml"

# Rollback state
SNAPSHOT_FILE="/tmp/nc-deploy-snapshot-$$"
ROLLBACK_ATTEMPTED=false

# Change to deployment directory
cd "$DEPLOY_DIR" || {
  echo -e "${RED}ERROR: Failed to change to deployment directory: $DEPLOY_DIR${NC}" >&2
  exit 1
}

echo -e "${GREEN}=== North Cloud Deployment Script ===${NC}"
echo "Deployment directory: $DEPLOY_DIR"
echo "Timestamp: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"

# Services that are built but not deployed via docker-compose
# (currently none — mcp-north-cloud runs in compose despite being stdio-based)
NON_COMPOSE_SERVICES=""

# Parse changed services
if [ -n "${CHANGED_SERVICES:-}" ]; then
  # Convert comma-separated to space-separated, filtering out non-compose services
  SERVICES_TO_UPDATE=""
  for svc in $(echo "$CHANGED_SERVICES" | tr ',' ' '); do
    if echo "$NON_COMPOSE_SERVICES" | grep -qw "$svc"; then
      echo -e "${BLUE}Skipping $svc (build-only, not a compose service)${NC}"
    else
      SERVICES_TO_UPDATE="$SERVICES_TO_UPDATE $svc"
    fi
  done
  SERVICES_TO_UPDATE=$(echo "$SERVICES_TO_UPDATE" | xargs)  # trim whitespace
  if [ -n "$SERVICES_TO_UPDATE" ]; then
    echo -e "${BLUE}Selective deployment: $SERVICES_TO_UPDATE${NC}"
  else
    echo -e "${GREEN}No compose services to deploy (all changes were build-only). Done.${NC}"
    exit 0
  fi
else
  SERVICES_TO_UPDATE=""
  echo -e "${BLUE}Full deployment: all services${NC}"
fi

echo ""

# Source environment variables from .env file
if [ -f .env ]; then
  echo -e "${GREEN}Loading environment variables from .env...${NC}"
  set -a
  source .env
  set +a
else
  echo -e "${YELLOW}WARNING: .env file not found. Some operations may fail.${NC}"
fi

# ============================================================
# Rollback Functions
# ============================================================

# Map compose service name to container name
container_name_for() {
  local service="$1"
  echo "north-cloud-${service}-1"
}

# Snapshot current image IDs for services being updated.
# Writes service=image_id pairs to SNAPSHOT_FILE.
snapshot_images() {
  local services="$1"
  echo -e "${BLUE}Snapshotting current image versions...${NC}"

  # Determine which services to snapshot
  local svc_list
  if [ -n "$services" ]; then
    svc_list="$services"
  else
    svc_list="auth crawler source-manager classifier publisher index-manager pipeline search-service dashboard"
  fi

  rm -f "$SNAPSHOT_FILE"
  touch "$SNAPSHOT_FILE"

  local snapshot_count=0
  for svc in $svc_list; do
    local container
    container=$(container_name_for "$svc")

    # Skip if container doesn't exist (first deploy)
    if ! docker inspect "$container" >/dev/null 2>&1; then
      continue
    fi

    # Capture the image ID (sha256 digest) currently running
    local image_id
    image_id=$(docker inspect --format '{{.Image}}' "$container" 2>/dev/null || true)

    # Capture the image name (e.g., docker.io/jonesrussell/crawler:latest)
    local image_name
    image_name=$(docker inspect --format '{{.Config.Image}}' "$container" 2>/dev/null || true)

    if [ -n "$image_id" ] && [ -n "$image_name" ]; then
      echo "${svc}|${image_name}|${image_id}" >> "$SNAPSHOT_FILE"
      snapshot_count=$((snapshot_count + 1))
    fi
  done

  echo -e "${GREEN}  Snapshotted $snapshot_count service(s)${NC}"
}

# Rollback failed services to their previous image versions.
# Reads from SNAPSHOT_FILE, re-tags old images, restarts services.
rollback_services() {
  local failed_services="$1"
  ROLLBACK_ATTEMPTED=true

  echo ""
  echo -e "${RED}=== ROLLBACK: Reverting failed services ===${NC}"

  if [ ! -f "$SNAPSHOT_FILE" ]; then
    echo -e "${RED}No snapshot file found - cannot rollback${NC}" >&2
    return 1
  fi

  local rollback_count=0
  local services_to_restart=""

  for svc in $failed_services; do
    # Find this service in the snapshot
    local snapshot_line
    snapshot_line=$(grep "^${svc}|" "$SNAPSHOT_FILE" 2>/dev/null || true)

    if [ -z "$snapshot_line" ]; then
      echo -e "${YELLOW}  No snapshot for $svc - skipping rollback${NC}"
      continue
    fi

    local image_name image_id
    image_name=$(echo "$snapshot_line" | cut -d'|' -f2)
    image_id=$(echo "$snapshot_line" | cut -d'|' -f3)

    echo -e "  Reverting $svc: tagging $image_id as $image_name"

    if docker tag "$image_id" "$image_name" 2>/dev/null; then
      services_to_restart="$services_to_restart $svc"
      rollback_count=$((rollback_count + 1))
    else
      echo -e "${RED}  Failed to re-tag image for $svc${NC}"
    fi
  done

  if [ -z "$services_to_restart" ]; then
    echo -e "${RED}No services could be rolled back${NC}" >&2
    return 1
  fi

  echo ""
  echo -e "${YELLOW}Restarting rolled-back services:$services_to_restart${NC}"
  if $COMPOSE_CMD up -d $services_to_restart 2>&1; then
    echo -e "${GREEN}  Rolled back $rollback_count service(s)${NC}"
  else
    echo -e "${RED}  Failed to restart rolled-back services${NC}" >&2
    return 1
  fi

  # Verify rolled-back services are healthy
  echo ""
  echo -e "${YELLOW}Verifying rollback health...${NC}"
  sleep 5

  local rollback_healthy=true
  for svc in $services_to_restart; do
    case "$svc" in
      auth)          check_health "auth" "/health" "8040" 10 || rollback_healthy=false ;;
      crawler)       check_health "crawler" "/health" "8060" 20 || rollback_healthy=false ;;
      source-manager) check_health "source-manager" "/health" "8050" 10 || rollback_healthy=false ;;
      classifier)    check_health "classifier" "/health" "8070" 10 || rollback_healthy=false ;;
      publisher)     check_health "publisher" "/health" "8070" 10 || rollback_healthy=false ;;
      index-manager) check_health "index-manager" "/health" "8090" 10 || rollback_healthy=false ;;
      pipeline)      check_health "pipeline" "/health" "8075" 10 || rollback_healthy=false ;;
      search-service) check_health "search-service" "/health" "8090" 10 || rollback_healthy=false ;;
    esac
  done

  if [ "$rollback_healthy" = true ]; then
    echo -e "${GREEN}  Rollback successful - services restored to previous version${NC}"
  else
    echo -e "${RED}  Rollback completed but some services still unhealthy${NC}" >&2
  fi

  return 0
}

# Cleanup snapshot file on exit
cleanup() {
  rm -f "$SNAPSHOT_FILE"
}
trap cleanup EXIT

# ============================================================
# Step 0: Login to Docker Hub
# ============================================================

if [ -n "${DOCKERHUB_USERNAME:-}" ] && [ -n "${DOCKERHUB_PASSWORD:-}" ]; then
  echo -e "${GREEN}Step 0: Logging in to Docker Hub...${NC}"
  echo "$DOCKERHUB_PASSWORD" | docker login -u "$DOCKERHUB_USERNAME" --password-stdin || {
    echo -e "${RED}ERROR: Failed to login to Docker Hub${NC}" >&2
    exit 1
  }
  echo -e "${GREEN}✓ Docker Hub login successful${NC}"
  echo ""
else
  echo -e "${YELLOW}WARNING: DOCKERHUB_USERNAME or DOCKERHUB_PASSWORD not set. Private images may fail to pull.${NC}"
  echo ""
fi

# ============================================================
# Step 1: Snapshot current images, then pull new ones
# ============================================================

echo -e "${GREEN}Step 1: Pulling Docker images...${NC}"

# Snapshot BEFORE pulling so we can rollback
snapshot_images "$SERVICES_TO_UPDATE"

if [ -n "$SERVICES_TO_UPDATE" ]; then
  echo "Pulling images for: $SERVICES_TO_UPDATE"
  if $COMPOSE_CMD pull $SERVICES_TO_UPDATE; then
    echo -e "${GREEN}✓ Selected images pulled successfully${NC}"
  else
    echo -e "${YELLOW}WARNING: Some images failed to pull. Continuing...${NC}"
  fi
else
  if $COMPOSE_CMD pull; then
    echo -e "${GREEN}✓ All images pulled successfully${NC}"
  else
    echo -e "${YELLOW}WARNING: Some images failed to pull. Continuing...${NC}"
  fi
fi
echo ""

# ============================================================
# Step 2: Run database migrations
# ============================================================

if [ "${MIGRATIONS_CHANGED:-true}" == "true" ] || [ -z "$SERVICES_TO_UPDATE" ]; then
  echo -e "${GREEN}Step 2: Running database migrations...${NC}"

  run_migration() {
    local service=$1
    local db_host=$2
    local db_port=$3
    local db_user=$4
    local db_password=$5
    local db_name=$6
    local migrations_path=$7

    echo -e "${YELLOW}Running migrations for $service...${NC}"

    if [ ! -d "$migrations_path" ] || [ -z "$(ls -A "$migrations_path" 2>/dev/null)" ]; then
      echo -e "${YELLOW}No migrations found for $service, skipping...${NC}"
      return 0
    fi

    local db_url="postgres://${db_user}:${db_password}@${db_host}:${db_port}/${db_name}?sslmode=disable"

    if ! docker run --rm --network north-cloud_north-cloud-network \
      -v "$DEPLOY_DIR/$migrations_path:/migrations" \
      migrate/migrate:latest \
      -path /migrations \
      -database "$db_url" \
      up; then
      echo -e "${RED}ERROR: Migration failed for $service${NC}" >&2
      return 1
    fi

    echo -e "${GREEN}✓ Migration completed for $service${NC}"
  }

  echo "Starting database services..."
  $COMPOSE_CMD up -d \
    postgres-crawler \
    postgres-source-manager \
    postgres-classifier \
    postgres-publisher \
    postgres-index-manager \
    postgres-pipeline || {
    echo -e "${RED}ERROR: Failed to start database services${NC}" >&2
    exit 1
  }

  echo "Waiting for databases to be ready..."
  for i in {1..30}; do
    if $COMPOSE_CMD exec -T postgres-crawler pg_isready -q 2>/dev/null; then
      echo "Databases are ready"
      break
    fi
    sleep 1
  done

  MIGRATION_PIDS=()

  if [ -n "${POSTGRES_SOURCE_MANAGER_USER:-}" ] && [ -n "${POSTGRES_SOURCE_MANAGER_PASSWORD:-}" ]; then
    run_migration "source-manager" "postgres-source-manager" "5432" \
      "$POSTGRES_SOURCE_MANAGER_USER" "$POSTGRES_SOURCE_MANAGER_PASSWORD" \
      "${POSTGRES_SOURCE_MANAGER_DB:-source_manager}" "source-manager/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  if [ -n "${POSTGRES_CRAWLER_USER:-}" ] && [ -n "${POSTGRES_CRAWLER_PASSWORD:-}" ]; then
    run_migration "crawler" "postgres-crawler" "5432" \
      "$POSTGRES_CRAWLER_USER" "$POSTGRES_CRAWLER_PASSWORD" \
      "${POSTGRES_CRAWLER_DB:-crawler}" "crawler/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  if [ -n "${POSTGRES_CLASSIFIER_USER:-}" ] && [ -n "${POSTGRES_CLASSIFIER_PASSWORD:-}" ]; then
    run_migration "classifier" "postgres-classifier" "5432" \
      "$POSTGRES_CLASSIFIER_USER" "$POSTGRES_CLASSIFIER_PASSWORD" \
      "${POSTGRES_CLASSIFIER_DB:-classifier}" "classifier/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  if [ -n "${POSTGRES_PUBLISHER_USER:-}" ] && [ -n "${POSTGRES_PUBLISHER_PASSWORD:-}" ]; then
    run_migration "publisher" "postgres-publisher" "5432" \
      "$POSTGRES_PUBLISHER_USER" "$POSTGRES_PUBLISHER_PASSWORD" \
      "${POSTGRES_PUBLISHER_DB:-publisher}" "publisher/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  if [ -n "${POSTGRES_INDEX_MANAGER_USER:-}" ] && [ -n "${POSTGRES_INDEX_MANAGER_PASSWORD:-}" ]; then
    run_migration "index-manager" "postgres-index-manager" "5432" \
      "$POSTGRES_INDEX_MANAGER_USER" "$POSTGRES_INDEX_MANAGER_PASSWORD" \
      "${POSTGRES_INDEX_MANAGER_DB:-index_manager}" "index-manager/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  if [ -n "${POSTGRES_PIPELINE_USER:-}" ] && [ -n "${POSTGRES_PIPELINE_PASSWORD:-}" ]; then
    run_migration "pipeline" "postgres-pipeline" "5432" \
      "$POSTGRES_PIPELINE_USER" "$POSTGRES_PIPELINE_PASSWORD" \
      "${POSTGRES_PIPELINE_DB:-pipeline}" "pipeline/migrations" &
    MIGRATION_PIDS+=($!)
  fi

  MIGRATION_FAILED=false
  for pid in "${MIGRATION_PIDS[@]}"; do
    if ! wait "$pid"; then
      MIGRATION_FAILED=true
    fi
  done

  if [ "$MIGRATION_FAILED" == "true" ]; then
    echo -e "${RED}ERROR: One or more migrations failed${NC}" >&2
    exit 1
  fi

  echo -e "${GREEN}✓ All migrations completed${NC}"
  echo ""
else
  echo -e "${YELLOW}Step 2: Skipping migrations (no migration changes detected)${NC}"
  echo ""
fi

# ============================================================
# Step 3: Restart services (with retry for transient failures)
# ============================================================

echo -e "${GREEN}Step 3: Restarting services...${NC}"

MAX_RESTART_ATTEMPTS=3
RESTART_WAIT_SECONDS=15

restart_with_retry() {
  local services="$1"
  local attempt=1

  while [ $attempt -le $MAX_RESTART_ATTEMPTS ]; do
    if [ $attempt -gt 1 ]; then
      echo -e "${YELLOW}Retry $attempt/$MAX_RESTART_ATTEMPTS (waiting ${RESTART_WAIT_SECONDS}s for dependencies)...${NC}"
      sleep "$RESTART_WAIT_SECONDS"
    fi

    if $COMPOSE_CMD up -d $services 2>&1; then
      return 0
    fi

    echo -e "${YELLOW}Attempt $attempt/$MAX_RESTART_ATTEMPTS failed${NC}"
    attempt=$((attempt + 1))
  done

  echo -e "${RED}ERROR: Failed to restart services after $MAX_RESTART_ATTEMPTS attempts${NC}" >&2
  return 1
}

if [ -n "$SERVICES_TO_UPDATE" ]; then
  echo "Restarting: $SERVICES_TO_UPDATE"
  restart_with_retry "$SERVICES_TO_UPDATE" || exit 1
else
  restart_with_retry "" || exit 1
fi
echo -e "${GREEN}✓ Services restarted${NC}"
echo ""

# ============================================================
# Step 3.5: Restart observability stack if infrastructure changed
# ============================================================

if [ "${INFRA_CHANGED:-false}" = "true" ] || [ -z "$SERVICES_TO_UPDATE" ]; then
  echo -e "${GREEN}Step 3.5: Restarting observability services (Alloy, Loki, Grafana)...${NC}"
  if $COMPOSE_CMD --profile observability up -d alloy loki grafana 2>&1; then
    echo -e "${GREEN}✓ Observability services restarted${NC}"
  else
    echo -e "${YELLOW}WARNING: Failed to restart observability services (profile may not be active)${NC}"
  fi
  echo ""
else
  echo -e "${YELLOW}Step 3.5: Skipping observability restart (no infrastructure changes)${NC}"
  echo ""
fi

# ============================================================
# Step 4: Health checks (with automatic rollback on failure)
# ============================================================

echo -e "${GREEN}Step 4: Performing health checks...${NC}"

check_health() {
  local service_name=$1
  local health_path=$2
  local port=$3
  local max_attempts=${4:-10}
  local attempt=1

  echo -n "  Checking $service_name... "

  while [ $attempt -le $max_attempts ]; do
    if $COMPOSE_CMD exec -T "$service_name" \
        wget -q -O /dev/null "http://localhost:${port}${health_path}" 2>/dev/null; then
      echo -e "${GREEN}✓${NC}"
      return 0
    fi
    sleep 3
    attempt=$((attempt + 1))
  done

  echo -e "${RED}✗ (failed after $max_attempts attempts)${NC}"
  return 1
}

# Determine which services to check
if [ -n "$SERVICES_TO_UPDATE" ]; then
  SERVICES_TO_CHECK="$SERVICES_TO_UPDATE"
else
  SERVICES_TO_CHECK="auth crawler source-manager classifier publisher index-manager pipeline search-service"
fi

FAILED_CHECKS=0
FAILED_SERVICES=""

for svc in $SERVICES_TO_CHECK; do
  case "$svc" in
    auth)
      check_health "auth" "/health" "8040" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    crawler)
      check_health "crawler" "/health" "8080" 20 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    source-manager)
      check_health "source-manager" "/health" "8050" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    classifier)
      check_health "classifier" "/health" "8070" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    publisher)
      check_health "publisher" "/health" "8070" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    index-manager)
      check_health "index-manager" "/health" "8090" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    pipeline)
      check_health "pipeline" "/health" "8075" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    search-service)
      check_health "search-service" "/health" "8090" 10 || { FAILED_CHECKS=$((FAILED_CHECKS + 1)); FAILED_SERVICES="$FAILED_SERVICES $svc"; }
      ;;
    # search-frontend and dashboard don't have health endpoints (static nginx)
  esac
done

if [ $FAILED_CHECKS -gt 0 ]; then
  echo ""
  echo -e "${RED}$FAILED_CHECKS service(s) failed health checks:$FAILED_SERVICES${NC}" >&2

  # Attempt automatic rollback
  if rollback_services "$FAILED_SERVICES"; then
    echo ""
    echo -e "${RED}=== Deployment FAILED (rolled back to previous version) ===${NC}" >&2
  else
    echo ""
    echo -e "${RED}=== Deployment FAILED (rollback also failed - manual intervention required) ===${NC}" >&2
  fi

  echo ""
  echo "Services status:"
  $COMPOSE_CMD ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null | head -25 || true
  exit 1
fi

echo ""

# ============================================================
# Step 5: Deployment summary
# ============================================================

echo -e "${GREEN}=== Deployment Summary ===${NC}"
echo "Completed at $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
if [ -n "$SERVICES_TO_UPDATE" ]; then
  echo "Updated services: $SERVICES_TO_UPDATE"
else
  echo "Updated services: all"
fi
echo ""
echo "Services status:"
# Use head -25 to avoid SIGPIPE when many containers (head -20 + pipefail causes script exit)
$COMPOSE_CMD ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null | head -25 || true
echo ""
echo -e "${GREEN}Deployment completed!${NC}"

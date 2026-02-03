#!/bin/bash
# Production deployment script for North Cloud
# This script should be placed at /opt/north-cloud/deploy.sh on the production server
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

# Change to deployment directory
cd "$DEPLOY_DIR" || {
  echo -e "${RED}ERROR: Failed to change to deployment directory: $DEPLOY_DIR${NC}" >&2
  exit 1
}

echo -e "${GREEN}=== North Cloud Deployment Script ===${NC}"
echo "Deployment directory: $DEPLOY_DIR"
echo "Timestamp: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"

# Parse changed services
if [ -n "${CHANGED_SERVICES:-}" ]; then
  # Convert comma-separated to space-separated
  SERVICES_TO_UPDATE=$(echo "$CHANGED_SERVICES" | tr ',' ' ')
  echo -e "${BLUE}Selective deployment: $SERVICES_TO_UPDATE${NC}"
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

# Step 0: Login to Docker Hub (if credentials are provided)
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

# Step 1: Pull latest images (selective if CHANGED_SERVICES is set)
echo -e "${GREEN}Step 1: Pulling Docker images...${NC}"

if [ -n "$SERVICES_TO_UPDATE" ]; then
  # Pull only changed services
  echo "Pulling images for: $SERVICES_TO_UPDATE"
  if $COMPOSE_CMD pull $SERVICES_TO_UPDATE; then
    echo -e "${GREEN}✓ Selected images pulled successfully${NC}"
  else
    echo -e "${YELLOW}WARNING: Some images failed to pull. Continuing...${NC}"
  fi
else
  # Pull all images
  if $COMPOSE_CMD pull; then
    echo -e "${GREEN}✓ All images pulled successfully${NC}"
  else
    echo -e "${YELLOW}WARNING: Some images failed to pull. Continuing...${NC}"
  fi
fi
echo ""

# Step 2: Run database migrations (only if migrations changed or full deploy)
if [ "${MIGRATIONS_CHANGED:-true}" == "true" ] || [ -z "$SERVICES_TO_UPDATE" ]; then
  echo -e "${GREEN}Step 2: Running database migrations...${NC}"

  # Function to run migrations for a service
  run_migration() {
    local service=$1
    local db_host=$2
    local db_port=$3
    local db_user=$4
    local db_password=$5
    local db_name=$6
    local migrations_path=$7

    echo -e "${YELLOW}Running migrations for $service...${NC}"

    # Check if migrations directory exists and has files
    if [ ! -d "$migrations_path" ] || [ -z "$(ls -A "$migrations_path" 2>/dev/null)" ]; then
      echo -e "${YELLOW}No migrations found for $service, skipping...${NC}"
      return 0
    fi

    # Construct PostgreSQL connection URL
    local db_url="postgres://${db_user}:${db_password}@${db_host}:${db_port}/${db_name}?sslmode=disable"

    # Run migrations
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

  # Ensure database services are running for migrations
  echo "Starting database services..."
  $COMPOSE_CMD up -d \
    postgres-crawler \
    postgres-source-manager \
    postgres-classifier \
    postgres-publisher \
    postgres-index-manager || {
    echo -e "${RED}ERROR: Failed to start database services${NC}" >&2
    exit 1
  }

  # Wait for databases to be ready (with health check polling)
  echo "Waiting for databases to be ready..."
  for i in {1..30}; do
    if $COMPOSE_CMD exec -T postgres-crawler pg_isready -q 2>/dev/null; then
      echo "Databases are ready"
      break
    fi
    sleep 1
  done

  # Run migrations in parallel (they use separate databases)
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

  # Wait for all migrations to complete
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

# Step 3: Restart services (selective if CHANGED_SERVICES is set)
echo -e "${GREEN}Step 3: Restarting services...${NC}"

if [ -n "$SERVICES_TO_UPDATE" ]; then
  # Restart only changed services
  echo "Restarting: $SERVICES_TO_UPDATE"
  $COMPOSE_CMD up -d $SERVICES_TO_UPDATE || {
    echo -e "${RED}ERROR: Failed to restart services${NC}" >&2
    exit 1
  }
else
  # Restart all services
  $COMPOSE_CMD up -d || {
    echo -e "${RED}ERROR: Failed to restart services${NC}" >&2
    exit 1
  }
fi
echo -e "${GREEN}✓ Services restarted${NC}"
echo ""

# Step 4: Health checks (poll instead of static sleep)
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
  SERVICES_TO_CHECK="auth crawler source-manager classifier publisher index-manager search-service"
fi

FAILED_CHECKS=0

for svc in $SERVICES_TO_CHECK; do
  case "$svc" in
    auth)
      check_health "auth" "/health" "8040" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    crawler)
      check_health "crawler" "/health" "8080" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    source-manager)
      check_health "source-manager" "/health" "8050" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    classifier)
      check_health "classifier" "/health" "8070" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    publisher)
      check_health "publisher" "/health" "8070" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    index-manager)
      check_health "index-manager" "/health" "8090" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    search-service)
      check_health "search-service" "/health" "8090" 10 || FAILED_CHECKS=$((FAILED_CHECKS + 1))
      ;;
    # search-frontend and dashboard don't have health endpoints (static nginx)
  esac
done

if [ $FAILED_CHECKS -gt 0 ]; then
  echo -e "${RED}WARNING: $FAILED_CHECKS service(s) failed health checks${NC}" >&2
  # Don't exit - services might still be starting
fi

echo ""

# Step 5: Deployment summary
echo -e "${GREEN}=== Deployment Summary ===${NC}"
echo "Completed at $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
if [ -n "$SERVICES_TO_UPDATE" ]; then
  echo "Updated services: $SERVICES_TO_UPDATE"
else
  echo "Updated services: all"
fi
echo ""
echo "Services status:"
$COMPOSE_CMD ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" | head -20
echo ""
echo -e "${GREEN}Deployment completed!${NC}"

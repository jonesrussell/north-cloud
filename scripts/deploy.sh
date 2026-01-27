#!/bin/bash
# Production deployment script for North Cloud
# This script should be placed at /opt/north-cloud/deploy.sh on the production server

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Deployment directory
DEPLOY_DIR="/opt/north-cloud"

# Change to deployment directory
cd "$DEPLOY_DIR" || {
  echo -e "${RED}ERROR: Failed to change to deployment directory: $DEPLOY_DIR${NC}" >&2
  exit 1
}

echo -e "${GREEN}=== North Cloud Deployment Script ===${NC}"
echo "Deployment directory: $DEPLOY_DIR"
echo "Timestamp: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
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

# Step 1: Pull latest images
echo -e "${GREEN}Step 1: Pulling latest Docker images...${NC}"
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml pull || {
  echo -e "${RED}ERROR: Failed to pull Docker images${NC}" >&2
  exit 1
}
echo -e "${GREEN}✓ Images pulled successfully${NC}"
echo ""

# Step 2: Run database migrations
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

  # Get current migration version (if any)
  local current_version
  current_version=$(docker run --rm --network north-cloud_north-cloud-network \
    migrate/migrate:latest \
    -path /migrations \
    -database "$db_url" \
    version 2>/dev/null | grep -o '[0-9]*' | head -1 || echo "0")

  echo "Current migration version for $service: $current_version"

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

  # Get new version
  local new_version
  new_version=$(docker run --rm --network north-cloud_north-cloud-network \
    migrate/migrate:latest \
    -path /migrations \
    -database "$db_url" \
    version 2>/dev/null | grep -o '[0-9]*' | head -1 || echo "0")

  echo "New migration version for $service: $new_version"
  echo -e "${GREEN}✓ Migration completed successfully for $service${NC}"
  echo ""
}

# Ensure infrastructure services are running for migrations
echo "Starting infrastructure services (databases)..."
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d \
  postgres-crawler \
  postgres-source-manager \
  postgres-classifier \
  postgres-publisher \
  postgres-index-manager || {
  echo -e "${RED}ERROR: Failed to start database services${NC}" >&2
  exit 1
}

# Wait for databases to be ready
echo "Waiting for databases to be ready..."
sleep 10

# Run migrations in exact order (as verified)
# 1. source-manager
if [ -n "${DB_USER:-}" ] && [ -n "${DB_PASSWORD:-}" ]; then
  run_migration "source-manager" \
    "${DB_HOST:-postgres-source-manager}" \
    "${DB_PORT:-5432}" \
    "$DB_USER" \
    "$DB_PASSWORD" \
    "${DB_NAME:-source_manager}" \
    "source-manager/migrations"
else
  echo -e "${YELLOW}WARNING: DB_USER or DB_PASSWORD not set, skipping source-manager migrations${NC}"
fi

# 2. crawler
if [ -n "${POSTGRES_CRAWLER_USER:-}" ] && [ -n "${POSTGRES_CRAWLER_PASSWORD:-}" ]; then
  run_migration "crawler" \
    "${POSTGRES_CRAWLER_HOST:-postgres-crawler}" \
    "${POSTGRES_CRAWLER_PORT:-5432}" \
    "$POSTGRES_CRAWLER_USER" \
    "$POSTGRES_CRAWLER_PASSWORD" \
    "${POSTGRES_CRAWLER_DB:-crawler}" \
    "crawler/migrations"
else
  echo -e "${YELLOW}WARNING: POSTGRES_CRAWLER_USER or POSTGRES_CRAWLER_PASSWORD not set, skipping crawler migrations${NC}"
fi

# 3. classifier
if [ -n "${POSTGRES_USER:-}" ] && [ -n "${POSTGRES_PASSWORD:-}" ]; then
  run_migration "classifier" \
    "${POSTGRES_HOST:-postgres-classifier}" \
    "${POSTGRES_PORT:-5432}" \
    "$POSTGRES_USER" \
    "$POSTGRES_PASSWORD" \
    "${POSTGRES_DB:-classifier}" \
    "classifier/migrations"
else
  echo -e "${YELLOW}WARNING: POSTGRES_USER or POSTGRES_PASSWORD not set, skipping classifier migrations${NC}"
fi

# 4. publisher
if [ -n "${POSTGRES_PUBLISHER_USER:-}" ] && [ -n "${POSTGRES_PUBLISHER_PASSWORD:-}" ]; then
  run_migration "publisher" \
    "${POSTGRES_PUBLISHER_HOST:-postgres-publisher}" \
    "${POSTGRES_PUBLISHER_PORT:-5432}" \
    "$POSTGRES_PUBLISHER_USER" \
    "$POSTGRES_PUBLISHER_PASSWORD" \
    "${POSTGRES_PUBLISHER_DB:-publisher}" \
    "publisher/migrations"
else
  echo -e "${YELLOW}WARNING: POSTGRES_PUBLISHER_USER or POSTGRES_PUBLISHER_PASSWORD not set, skipping publisher migrations${NC}"
fi

# 5. index-manager
if [ -n "${POSTGRES_INDEX_MANAGER_USER:-}" ] && [ -n "${POSTGRES_INDEX_MANAGER_PASSWORD:-}" ]; then
  run_migration "index-manager" \
    "${POSTGRES_INDEX_MANAGER_HOST:-postgres-index-manager}" \
    "${POSTGRES_INDEX_MANAGER_PORT:-5432}" \
    "$POSTGRES_INDEX_MANAGER_USER" \
    "$POSTGRES_INDEX_MANAGER_PASSWORD" \
    "${POSTGRES_INDEX_MANAGER_DB:-index_manager}" \
    "index-manager/migrations"
else
  echo -e "${YELLOW}WARNING: POSTGRES_INDEX_MANAGER_USER or POSTGRES_INDEX_MANAGER_PASSWORD not set, skipping index-manager migrations${NC}"
fi

echo -e "${GREEN}✓ All migrations completed successfully${NC}"
echo ""

# Step 3: Restart services
echo -e "${GREEN}Step 3: Restarting services...${NC}"
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d || {
  echo -e "${RED}ERROR: Failed to restart services${NC}" >&2
  exit 1
}
echo -e "${GREEN}✓ Services restarted${NC}"
echo ""

# Step 4: Wait for services to start
echo -e "${GREEN}Step 4: Waiting for services to start (30 seconds)...${NC}"
sleep 30
echo ""

# Step 5: Health checks
echo -e "${GREEN}Step 5: Performing health checks...${NC}"

# Function to check health endpoint
check_health() {
  local service_name=$1
  local health_url=$2
  local max_attempts=${3:-5}
  local attempt=1

  echo -e "${YELLOW}Checking health for $service_name at $health_url...${NC}"

  while [ $attempt -le $max_attempts ]; do
    if curl -f -s "$health_url" > /dev/null 2>&1; then
      echo -e "${GREEN}✓ $service_name is healthy${NC}"
      return 0
    fi

    if [ $attempt -lt $max_attempts ]; then
      echo "Attempt $attempt/$max_attempts: $service_name not ready yet, waiting 10 seconds..."
      sleep 10
    fi
    attempt=$((attempt + 1))
  done

  echo -e "${RED}✗ $service_name health check failed after $max_attempts attempts${NC}"
  return 1
}

# Check backend services (using verified endpoints)
FAILED_CHECKS=0

# auth: GET /health on port 8040
if ! check_health "auth" "http://localhost:8040/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# crawler: GET /health on port 8080 (internal)
if ! check_health "crawler" "http://localhost:8080/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# source-manager: GET /health on port 8050
if ! check_health "source-manager" "http://localhost:8050/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# classifier: GET /health on port 8070
if ! check_health "classifier" "http://localhost:8070/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# publisher: GET /health on port 8070
if ! check_health "publisher" "http://localhost:8070/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# index-manager: GET /health on port 8090
if ! check_health "index-manager" "http://localhost:8090/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# search-service: GET /health on port 8090
if ! check_health "search-service" "http://localhost:8090/health" 5; then
  FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi

# search-frontend and dashboard: Skip (static nginx, no HTTP health endpoints)

if [ $FAILED_CHECKS -gt 0 ]; then
  echo -e "${RED}ERROR: $FAILED_CHECKS service(s) failed health checks${NC}" >&2
  exit 1
fi

echo -e "${GREEN}✓ All health checks passed successfully${NC}"
echo ""

# Step 6: Deployment summary
echo -e "${GREEN}=== Deployment Summary ===${NC}"
echo "Deployment completed successfully at $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
echo ""
echo "Services status:"
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml ps
echo ""
echo -e "${GREEN}Deployment completed successfully!${NC}"

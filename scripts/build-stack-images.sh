#!/bin/bash
# Build all Docker images required for Docker Swarm stack deployment
# This script builds images that will be used by docker stack deploy

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "Building Docker images for stack deployment..."
echo ""

# Build search-service
echo "Building search-service..."
docker build -t northcloud/search-service:latest \
  -f ./search/Dockerfile \
  .

# Build search-frontend
echo "Building search-frontend..."
docker build -t northcloud/search-frontend:latest \
  -f ./search-frontend/Dockerfile \
  ./search-frontend

# Build auth
echo "Building auth..."
docker build -t northcloud/auth:latest \
  -f ./auth/Dockerfile \
  .

# Build crawler
echo "Building crawler..."
docker build -t northcloud/crawler:latest \
  --build-arg BUILD_ENV=production \
  -f ./crawler/Dockerfile \
  .

# Build source-manager
echo "Building source-manager..."
docker build -t northcloud/source-manager:latest \
  --build-arg BUILD_ENV=production \
  -f ./source-manager/Dockerfile \
  .

# Build publisher
echo "Building publisher..."
docker build -t northcloud/publisher:latest \
  --build-arg BUILD_ENV=production \
  -f ./publisher/Dockerfile \
  .

# Build classifier
echo "Building classifier..."
docker build -t northcloud/classifier:latest \
  --build-arg BUILD_ENV=production \
  -f ./classifier/Dockerfile \
  .

# Build dashboard
echo "Building dashboard..."
docker build -t northcloud/dashboard:latest \
  --build-arg BUILD_ENV=production \
  -f ./dashboard/Dockerfile \
  ./dashboard

# Build index-manager (if it has a Dockerfile)
if [ -f "./index-manager/Dockerfile" ]; then
  echo "Building index-manager..."
  docker build -t northcloud/index-manager:latest \
    -f ./index-manager/Dockerfile \
    ./index-manager
fi

echo ""
echo "All images built successfully!"
echo ""
echo "To deploy the stack, run:"
echo "  docker stack deploy -c docker-compose.base.yml -c docker-compose.prod.yml northcloud"
echo ""
echo "Note: Make sure to remove 'build' directives from compose files for stack deployment."

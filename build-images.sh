#!/bin/bash
# Build all Docker images for Docker Swarm deployment
# Usage: ./build-images.sh

set -e

echo "Building Docker images for North Cloud stack..."

# Build images with the same names used in docker-compose files
docker build -t northcloud/search-service:latest -f ./search/Dockerfile .
docker build -t northcloud/search-frontend:latest -f ./search-frontend/Dockerfile ./search-frontend
docker build -t northcloud/auth:latest -f ./auth/Dockerfile ./auth
docker build -t northcloud/crawler:latest --build-arg BUILD_ENV=production -f ./crawler/Dockerfile .
docker build -t northcloud/source-manager:latest --build-arg BUILD_ENV=production -f ./source-manager/Dockerfile .
docker build -t northcloud/publisher:latest --build-arg BUILD_ENV=production -f ./publisher/Dockerfile .
docker build -t northcloud/classifier:latest --build-arg BUILD_ENV=production -f ./classifier/Dockerfile .
docker build -t northcloud/index-manager:latest --build-arg BUILD_ENV=production -f ./index-manager/Dockerfile ./index-manager
docker build -t northcloud/dashboard:latest --build-arg BUILD_ENV=production -f ./dashboard/Dockerfile ./dashboard

echo "All images built successfully!"
echo ""
echo "You can now deploy the stack with:"
echo "  docker stack deploy -c docker-compose.base.yml -c docker-compose.prod.yml northcloud"

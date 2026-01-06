#!/bin/bash

# ============================================================
# Production Build Script
# ============================================================
# Builds each service one at a time for production
# Usage: ./scripts/build-prod.sh [service-name]
#   If service-name is provided, only that service is built
#   Otherwise, all services are built sequentially
# ============================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Compose files for production
COMPOSE_FILES="-f docker-compose.base.yml -f docker-compose.prod.yml"

# Services to build (in order)
SERVICES=(
  "auth"
  "crawler"
  "source-manager"
  "classifier"
  "index-manager"
  "publisher-api"  # publisher-api and publisher-router share the same image
  "search-service"
  "dashboard"
)

# Function to print colored output
print_status() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to build a single service
build_service() {
  local service=$1
  local start_time=$(date +%s)
  
  print_status "Building service: ${BLUE}$service${NC}"
  echo "----------------------------------------"
  
  if docker compose $COMPOSE_FILES build "$service" 2>&1; then
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    print_success "Service $service built successfully in ${duration}s"
    return 0
  else
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    print_error "Failed to build service $service after ${duration}s"
    return 1
  fi
}

# Main execution
main() {
  local total_start_time=$(date +%s)
  local services_to_build=("${SERVICES[@]}")
  local failed_services=()
  local success_count=0
  local fail_count=0
  
  print_status "Starting production build process"
  print_status "Project root: $PROJECT_ROOT"
  echo ""
  
  # If a specific service is requested, build only that
  if [ $# -gt 0 ]; then
    local requested_service=$1
    if [[ " ${SERVICES[@]} " =~ " ${requested_service} " ]]; then
      services_to_build=("$requested_service")
      print_status "Building single service: $requested_service"
    else
      print_error "Unknown service: $requested_service"
      print_status "Available services: ${SERVICES[*]}"
      exit 1
    fi
  else
    print_status "Building all ${#SERVICES[@]} services sequentially"
  fi
  
  echo ""
  
  # Build each service
  for service in "${services_to_build[@]}"; do
    if build_service "$service"; then
      ((success_count++))
      echo ""
    else
      ((fail_count++))
      failed_services+=("$service")
      echo ""
      
      # Ask if user wants to continue
      read -p "Continue with remaining services? (y/n): " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_warning "Build process stopped by user"
        break
      fi
    fi
  done
  
  # Summary
  local total_end_time=$(date +%s)
  local total_duration=$((total_end_time - total_start_time))
  
  echo ""
  echo "========================================"
  print_status "Build Summary"
  echo "========================================"
  print_success "Successfully built: $success_count service(s)"
  
  if [ $fail_count -gt 0 ]; then
    print_error "Failed: $fail_count service(s)"
    print_error "Failed services: ${failed_services[*]}"
    exit 1
  else
    print_success "All services built successfully!"
    print_status "Total time: ${total_duration}s"
    exit 0
  fi
}

# Run main function
main "$@"

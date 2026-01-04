#!/bin/bash
# profile.sh - Automated profile capture for North Cloud services
# Usage: ./scripts/profile.sh [service] [profile_type] [duration]
#   service: crawler, source-manager, classifier, publisher-api, publisher-router, auth, search
#   profile_type: heap, cpu, goroutine, allocs, block, mutex
#   duration: profile duration in seconds (default: 30 for CPU, ignored for others)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
DURATION=30
OUTPUT_DIR="./profiles"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Service port mapping
declare -A SERVICE_PORTS=(
    ["crawler"]=6060
    ["source-manager"]=6061
    ["classifier"]=6062
    ["publisher-api"]=6063
    ["publisher-router"]=6064
    ["auth"]=6065
    ["search"]=6066
)

# Help message
usage() {
    echo "Usage: $0 [service] [profile_type] [duration]"
    echo ""
    echo "Arguments:"
    echo "  service       Service name (crawler, source-manager, classifier, publisher-api, publisher-router, auth, search)"
    echo "  profile_type  Profile type (heap, cpu, goroutine, allocs, block, mutex)"
    echo "  duration      Profile duration in seconds (default: 30 for CPU, ignored for others)"
    echo ""
    echo "Examples:"
    echo "  $0 crawler heap                    # Capture heap profile"
    echo "  $0 publisher-api cpu 60            # Capture 60s CPU profile"
    echo "  $0 classifier goroutine            # Capture goroutine profile"
    echo "  $0 search allocs                   # Capture allocation profile"
    exit 1
}

# Validate arguments
if [ $# -lt 2 ]; then
    echo -e "${RED}Error: Missing required arguments${NC}"
    usage
fi

SERVICE=$1
PROFILE_TYPE=$2
DURATION=${3:-30}

# Validate service
if [ -z "${SERVICE_PORTS[$SERVICE]}" ]; then
    echo -e "${RED}Error: Invalid service '$SERVICE'${NC}"
    echo "Valid services: ${!SERVICE_PORTS[@]}"
    exit 1
fi

# Validate profile type
VALID_TYPES=("heap" "cpu" "goroutine" "allocs" "block" "mutex")
if [[ ! " ${VALID_TYPES[@]} " =~ " ${PROFILE_TYPE} " ]]; then
    echo -e "${RED}Error: Invalid profile type '$PROFILE_TYPE'${NC}"
    echo "Valid types: ${VALID_TYPES[@]}"
    exit 1
fi

PORT=${SERVICE_PORTS[$SERVICE]}

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build output filename
OUTPUT_FILE="${OUTPUT_DIR}/${SERVICE}_${PROFILE_TYPE}_${TIMESTAMP}.pb.gz"

echo -e "${GREEN}Capturing ${PROFILE_TYPE} profile for ${SERVICE}...${NC}"
echo "Service: $SERVICE"
echo "Port: $PORT"
echo "Profile Type: $PROFILE_TYPE"
echo "Output: $OUTPUT_FILE"

# Capture profile based on type
if [ "$PROFILE_TYPE" == "cpu" ]; then
    echo "Duration: ${DURATION}s"
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/profile?seconds=${DURATION}" -o "$OUTPUT_FILE"
elif [ "$PROFILE_TYPE" == "heap" ]; then
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/heap" -o "$OUTPUT_FILE"
elif [ "$PROFILE_TYPE" == "goroutine" ]; then
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/goroutine" -o "$OUTPUT_FILE"
elif [ "$PROFILE_TYPE" == "allocs" ]; then
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/allocs" -o "$OUTPUT_FILE"
elif [ "$PROFILE_TYPE" == "block" ]; then
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/block" -o "$OUTPUT_FILE"
elif [ "$PROFILE_TYPE" == "mutex" ]; then
    echo ""
    curl -sS "http://localhost:${PORT}/debug/pprof/mutex" -o "$OUTPUT_FILE"
fi

# Check if profile was captured successfully
if [ -f "$OUTPUT_FILE" ] && [ -s "$OUTPUT_FILE" ]; then
    FILE_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
    echo -e "${GREEN}✓ Profile captured successfully${NC}"
    echo "File: $OUTPUT_FILE"
    echo "Size: $FILE_SIZE"
    echo ""
    echo -e "${YELLOW}Analyze with:${NC}"
    echo "  go tool pprof $OUTPUT_FILE"
    echo "  go tool pprof -http=:8080 $OUTPUT_FILE"
else
    echo -e "${RED}✗ Failed to capture profile${NC}"
    echo "Check that the service is running and pprof is enabled"
    exit 1
fi

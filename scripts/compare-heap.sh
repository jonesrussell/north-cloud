#!/bin/bash
# compare-heap.sh - Memory leak detection via heap profile comparison
# Usage: ./scripts/compare-heap.sh [service] [interval]
#   service: crawler, source-manager, classifier, publisher-api, publisher-router, auth, search
#   interval: time between captures in seconds (default: 300)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
INTERVAL=300
OUTPUT_DIR="./profiles/heap_comparison"
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
    echo "Usage: $0 [service] [interval]"
    echo ""
    echo "Arguments:"
    echo "  service   Service name (crawler, source-manager, classifier, publisher-api, publisher-router, auth, search)"
    echo "  interval  Time between captures in seconds (default: 300)"
    echo ""
    echo "Examples:"
    echo "  $0 crawler 600           # Compare heap profiles 10 minutes apart"
    echo "  $0 publisher-api 300     # Compare heap profiles 5 minutes apart"
    echo ""
    echo "This script captures two heap profiles separated by the interval and compares them"
    echo "to detect memory leaks. Significant growth indicates potential leaks."
    exit 1
}

# Validate arguments
if [ $# -lt 1 ]; then
    echo -e "${RED}Error: Missing required service argument${NC}"
    usage
fi

SERVICE=$1
INTERVAL=${2:-300}

# Validate service
if [ -z "${SERVICE_PORTS[$SERVICE]}" ]; then
    echo -e "${RED}Error: Invalid service '$SERVICE'${NC}"
    echo "Valid services: ${!SERVICE_PORTS[@]}"
    exit 1
fi

PORT=${SERVICE_PORTS[$SERVICE]}

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build output filenames
BASELINE="${OUTPUT_DIR}/${SERVICE}_baseline_${TIMESTAMP}.pb.gz"
CURRENT="${OUTPUT_DIR}/${SERVICE}_current_${TIMESTAMP}.pb.gz"

echo -e "${BLUE}=== Memory Leak Detection ===${NC}"
echo "Service: $SERVICE"
echo "Port: $PORT"
echo "Interval: ${INTERVAL}s ($(($INTERVAL / 60)) minutes)"
echo ""

# Capture baseline heap profile
echo -e "${GREEN}[1/3] Capturing baseline heap profile...${NC}"
curl -sS "http://localhost:${PORT}/debug/pprof/heap" -o "$BASELINE"

if [ ! -f "$BASELINE" ] || [ ! -s "$BASELINE" ]; then
    echo -e "${RED}✗ Failed to capture baseline profile${NC}"
    echo "Check that the service is running and pprof is enabled"
    exit 1
fi

BASELINE_SIZE=$(du -h "$BASELINE" | cut -f1)
echo -e "${GREEN}✓ Baseline captured${NC} ($BASELINE_SIZE)"
echo "File: $BASELINE"
echo ""

# Wait for interval
echo -e "${YELLOW}[2/3] Waiting ${INTERVAL}s for memory activity...${NC}"
echo "Service is running. This is a good time to:"
echo "  - Generate load on the service"
echo "  - Exercise key workflows"
echo "  - Trigger suspect operations"
echo ""

for ((i=$INTERVAL; i>0; i--)); do
    printf "\rTime remaining: %3ds" $i
    sleep 1
done
printf "\r"
echo ""

# Capture current heap profile
echo -e "${GREEN}[3/3] Capturing current heap profile...${NC}"
curl -sS "http://localhost:${PORT}/debug/pprof/heap" -o "$CURRENT"

if [ ! -f "$CURRENT" ] || [ ! -s "$CURRENT" ]; then
    echo -e "${RED}✗ Failed to capture current profile${NC}"
    exit 1
fi

CURRENT_SIZE=$(du -h "$CURRENT" | cut -f1)
echo -e "${GREEN}✓ Current profile captured${NC} ($CURRENT_SIZE)"
echo "File: $CURRENT"
echo ""

# Compare profiles
echo -e "${BLUE}=== Heap Growth Analysis ===${NC}"
echo ""
echo "Analyzing heap growth between captures..."
echo ""

# Use go tool pprof to compare profiles
echo -e "${YELLOW}Top 10 objects with most growth:${NC}"
go tool pprof -top -base="$BASELINE" "$CURRENT" 2>/dev/null | head -n 20

echo ""
echo -e "${YELLOW}Detailed comparison:${NC}"
echo "Run the following command for interactive analysis:"
echo "  go tool pprof -base='$BASELINE' '$CURRENT'"
echo ""
echo "Or view in browser:"
echo "  go tool pprof -http=:8080 -base='$BASELINE' '$CURRENT'"
echo ""

# Check for significant growth
echo -e "${BLUE}=== Memory Leak Assessment ===${NC}"
echo ""

# Extract heap sizes using go tool pprof
BASELINE_MB=$(go tool pprof -sample_index=alloc_space -top "$BASELINE" 2>/dev/null | grep "Total:" | awk '{print $2}' | sed 's/MB//')
CURRENT_MB=$(go tool pprof -sample_index=alloc_space -top "$CURRENT" 2>/dev/null | grep "Total:" | awk '{print $2}' | sed 's/MB//')

if [ -n "$BASELINE_MB" ] && [ -n "$CURRENT_MB" ]; then
    GROWTH=$(awk "BEGIN {printf \"%.2f\", ($CURRENT_MB - $BASELINE_MB)}")
    GROWTH_PCT=$(awk "BEGIN {printf \"%.1f\", (($CURRENT_MB - $BASELINE_MB) / $BASELINE_MB) * 100}")

    echo "Baseline heap: ${BASELINE_MB}MB"
    echo "Current heap:  ${CURRENT_MB}MB"
    echo "Growth:        ${GROWTH}MB (${GROWTH_PCT}%)"
    echo ""

    # Thresholds for leak detection
    if (( $(echo "$GROWTH_PCT > 50" | bc -l) )); then
        echo -e "${RED}⚠ WARNING: Significant heap growth detected (>50%)${NC}"
        echo "This may indicate a memory leak. Investigate with:"
        echo "  go tool pprof -base='$BASELINE' '$CURRENT'"
    elif (( $(echo "$GROWTH_PCT > 20" | bc -l) )); then
        echo -e "${YELLOW}⚡ CAUTION: Moderate heap growth detected (>20%)${NC}"
        echo "Monitor for continued growth over time"
    else
        echo -e "${GREEN}✓ Heap growth within normal range (<20%)${NC}"
    fi
else
    echo -e "${YELLOW}Note: Could not parse heap sizes automatically${NC}"
    echo "Review profiles manually using the commands above"
fi

echo ""
echo -e "${BLUE}Profiles saved to: $OUTPUT_DIR${NC}"

#!/bin/bash
# check-memory-leaks.sh - Automated memory leak detection across all services
# Usage: ./scripts/check-memory-leaks.sh [options]
#   -s, --service <name>     Check specific service only
#   -i, --interval <seconds> Time between checks (default: 300)
#   -c, --count <num>        Number of checks to perform (default: 3)
#   -t, --threshold <pct>    Growth threshold percentage (default: 50)
#   -a, --alert              Enable log-based alerts for leaks

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Default values
INTERVAL=300
CHECK_COUNT=3
THRESHOLD=50
ALERT_ENABLED=false
SPECIFIC_SERVICE=""
OUTPUT_DIR="./leak_detection"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="${OUTPUT_DIR}/leak_check_${TIMESTAMP}.log"

# Service port mapping (application ports, not pprof ports)
# Memory health endpoints are on the main application HTTP server
# Note: publisher-router is a background worker without HTTP server, so it's excluded
declare -A SERVICE_PORTS=(
    ["crawler"]=8060
    ["source-manager"]=8050
    ["classifier"]=8071
    ["publisher-api"]=8070
    ["auth"]=8040
    ["search"]=8092
)

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--service)
            SPECIFIC_SERVICE="$2"
            shift 2
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -c|--count)
            CHECK_COUNT="$2"
            shift 2
            ;;
        -t|--threshold)
            THRESHOLD="$2"
            shift 2
            ;;
        -a|--alert)
            ALERT_ENABLED=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -s, --service <name>     Check specific service only"
            echo "  -i, --interval <seconds> Time between checks (default: 300)"
            echo "  -c, --count <num>        Number of checks to perform (default: 3)"
            echo "  -t, --threshold <pct>    Growth threshold percentage (default: 50)"
            echo "  -a, --alert              Enable log-based alerts for leaks"
            echo ""
            echo "Services: crawler, source-manager, classifier, publisher-api, publisher-router, auth, search"
            echo ""
            echo "Examples:"
            echo "  $0                                # Check all services 3 times at 5min intervals"
            echo "  $0 -s crawler -i 600 -c 5         # Check crawler 5 times at 10min intervals"
            echo "  $0 -t 30 -a                       # Check with 30% threshold and alerts"
            echo "  $0 -s publisher-api -i 900 -c 10  # Extended check (15min × 10 = 2.5hrs)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Validate service if specified
if [ -n "$SPECIFIC_SERVICE" ] && [ -z "${SERVICE_PORTS[$SPECIFIC_SERVICE]}" ]; then
    echo -e "${RED}Error: Invalid service '$SPECIFIC_SERVICE'${NC}"
    echo "Valid services: ${!SERVICE_PORTS[@]}"
    exit 1
fi

# Determine which services to check
if [ -n "$SPECIFIC_SERVICE" ]; then
    SERVICES=("$SPECIFIC_SERVICE")
else
    SERVICES=("${!SERVICE_PORTS[@]}")
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Initialize log file
{
    echo "# Memory Leak Detection Report"
    echo "# Timestamp: $(date)"
    echo "# Services: ${SERVICES[*]}"
    echo "# Interval: ${INTERVAL}s ($(($INTERVAL / 60)) minutes)"
    echo "# Checks: $CHECK_COUNT"
    echo "# Threshold: ${THRESHOLD}%"
    echo "# Alert: $([ "$ALERT_ENABLED" = true ] && echo "enabled" || echo "disabled")"
    echo ""
} > "$LOG_FILE"

echo -e "${BLUE}=== Automated Memory Leak Detection ===${NC}"
echo "Services: ${SERVICES[*]}"
echo "Interval: ${INTERVAL}s ($(($INTERVAL / 60)) minutes)"
echo "Checks: $CHECK_COUNT"
echo "Threshold: ${THRESHOLD}%"
echo "Log file: $LOG_FILE"
echo ""

# Function to get memory health stats
get_memory_stats() {
    local service=$1
    local port=${SERVICE_PORTS[$service]}

    # Fetch memory health endpoint
    if ! curl -sS -f "http://localhost:${port}/health/memory" 2>/dev/null; then
        echo ""
        return 1
    fi
}

# Function to parse memory stats
parse_heap_alloc() {
    echo "$1" | grep -oP '"heap_alloc_mb":\s*\K[0-9.]+' || echo "0"
}

parse_num_goroutine() {
    echo "$1" | grep -oP '"num_goroutine":\s*\K[0-9]+' || echo "0"
}

# Function to log alert
log_alert() {
    local level=$1
    local message=$2

    echo "[$level] $(date): $message" >> "$LOG_FILE"

    if [ "$ALERT_ENABLED" = true ]; then
        case $level in
            ERROR)
                echo -e "${RED}[ALERT] $message${NC}" >&2
                ;;
            WARN)
                echo -e "${YELLOW}[ALERT] $message${NC}" >&2
                ;;
            INFO)
                echo -e "${GREEN}[ALERT] $message${NC}" >&2
                ;;
        esac
    fi
}

# Check each service
declare -A BASELINE_HEAP
declare -A BASELINE_GOROUTINES
declare -A LEAK_DETECTED

for service in "${SERVICES[@]}"; do
    echo -e "${MAGENTA}Checking service: $service${NC}"

    port=${SERVICE_PORTS[$service]}

    # Verify service is accessible
    if ! curl -sS -f "http://localhost:${port}/health/memory" > /dev/null 2>&1; then
        echo -e "${RED}✗ Service not accessible on port $port${NC}"
        log_alert "ERROR" "$service: Service not accessible"
        echo ""
        continue
    fi

    echo -e "${GREEN}✓ Service accessible${NC}"

    # Establish baseline
    echo "Establishing baseline..."
    stats=$(get_memory_stats "$service")
    if [ -z "$stats" ]; then
        echo -e "${RED}✗ Failed to get memory stats${NC}"
        log_alert "ERROR" "$service: Failed to get baseline stats"
        echo ""
        continue
    fi

    baseline_heap=$(parse_heap_alloc "$stats")
    baseline_goroutines=$(parse_num_goroutine "$stats")

    BASELINE_HEAP[$service]=$baseline_heap
    BASELINE_GOROUTINES[$service]=$baseline_goroutines

    echo "Baseline - Heap: ${baseline_heap}MB, Goroutines: $baseline_goroutines"
    log_alert "INFO" "$service: Baseline established - Heap: ${baseline_heap}MB, Goroutines: $baseline_goroutines"

    # Perform checks over time
    LEAK_DETECTED[$service]=false

    for ((check=1; check<=CHECK_COUNT; check++)); do
        echo ""
        echo -e "${YELLOW}Check $check/$CHECK_COUNT${NC}"

        if [ $check -gt 1 ]; then
            echo "Waiting ${INTERVAL}s..."
            for ((i=$INTERVAL; i>0; i--)); do
                printf "\rTime remaining: %3ds" $i
                sleep 1
            done
            printf "\r"
        fi

        # Get current stats
        stats=$(get_memory_stats "$service")
        if [ -z "$stats" ]; then
            echo -e "${RED}✗ Failed to get memory stats${NC}"
            log_alert "ERROR" "$service: Failed to get stats at check $check"
            continue
        fi

        current_heap=$(parse_heap_alloc "$stats")
        current_goroutines=$(parse_num_goroutine "$stats")

        # Calculate growth
        heap_growth=$(awk "BEGIN {printf \"%.2f\", $current_heap - $baseline_heap}")
        heap_growth_pct=$(awk "BEGIN {if ($baseline_heap > 0) printf \"%.1f\", ($current_heap - $baseline_heap) / $baseline_heap * 100; else print 0}")

        goroutine_growth=$(awk "BEGIN {printf \"%.0f\", $current_goroutines - $baseline_goroutines}")
        goroutine_growth_pct=$(awk "BEGIN {if ($baseline_goroutines > 0) printf \"%.1f\", ($current_goroutines - $baseline_goroutines) / $baseline_goroutines * 100; else print 0}")

        echo "Current - Heap: ${current_heap}MB (+${heap_growth}MB, ${heap_growth_pct}%), Goroutines: $current_goroutines (+${goroutine_growth}, ${goroutine_growth_pct}%)"

        # Check against threshold
        if (( $(echo "$heap_growth_pct > $THRESHOLD" | bc -l) )); then
            echo -e "${RED}⚠ LEAK DETECTED: Heap growth ${heap_growth_pct}% exceeds threshold ${THRESHOLD}%${NC}"
            log_alert "ERROR" "$service: Memory leak detected - Heap growth ${heap_growth_pct}% (${current_heap}MB)"
            LEAK_DETECTED[$service]=true
        fi

        if (( $(echo "$goroutine_growth_pct > $THRESHOLD" | bc -l) )); then
            echo -e "${RED}⚠ LEAK DETECTED: Goroutine growth ${goroutine_growth_pct}% exceeds threshold ${THRESHOLD}%${NC}"
            log_alert "ERROR" "$service: Goroutine leak detected - Growth ${goroutine_growth_pct}% ($current_goroutines goroutines)"
            LEAK_DETECTED[$service]=true
        fi

        # Log check result
        log_alert "INFO" "$service: Check $check - Heap: ${current_heap}MB (${heap_growth_pct}%), Goroutines: $current_goroutines (${goroutine_growth_pct}%)"
    done

    echo ""
done

# Final summary
echo -e "${BLUE}=== Leak Detection Summary ===${NC}"
echo ""

LEAKS_FOUND=false
for service in "${SERVICES[@]}"; do
    if [ "${LEAK_DETECTED[$service]}" = true ]; then
        echo -e "${RED}✗ $service: LEAK DETECTED${NC}"
        LEAKS_FOUND=true
    else
        echo -e "${GREEN}✓ $service: No leaks detected${NC}"
    fi
done

echo ""
echo "Full report: $LOG_FILE"

if [ "$LEAKS_FOUND" = true ]; then
    echo ""
    echo -e "${YELLOW}Action required:${NC}"
    echo "1. Review the log file for detailed growth metrics"
    echo "2. Capture heap profiles for affected services:"
    echo "   ./scripts/profile.sh <service> heap"
    echo "3. Compare heap profiles over time:"
    echo "   ./scripts/compare-heap.sh <service> 600"
    echo "4. Analyze with pprof:"
    echo "   go tool pprof -http=:8080 profiles/<service>_heap_*.pb.gz"
    exit 1
else
    echo ""
    echo -e "${GREEN}All services passed leak detection ✓${NC}"
    exit 0
fi

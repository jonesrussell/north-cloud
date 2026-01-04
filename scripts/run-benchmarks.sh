#!/bin/bash
# run-benchmarks.sh - Run benchmarks across all North Cloud services
# Usage: ./scripts/run-benchmarks.sh [options]
#   -s, --service <name>     Run benchmarks for specific service only
#   -b, --baseline           Save as baseline for future comparison
#   -c, --compare <file>     Compare against previous baseline
#   -v, --verbose            Show verbose benchmark output
#   -m, --benchmem           Include memory allocation statistics
#   -t, --benchtime <time>   Run each benchmark for specified time (default: 1s)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
BASELINE_DIR="./benchmarks/baselines"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BENCHTIME="1s"
BENCHMEM=""
VERBOSE=""
SPECIFIC_SERVICE=""
SAVE_BASELINE=false
COMPARE_FILE=""

# Service directories
declare -A SERVICE_DIRS=(
    ["crawler"]="./crawler"
    ["source-manager"]="./source-manager"
    ["classifier"]="./classifier"
    ["publisher"]="./publisher"
    ["auth"]="./auth"
    ["search"]="./search"
)

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--service)
            SPECIFIC_SERVICE="$2"
            shift 2
            ;;
        -b|--baseline)
            SAVE_BASELINE=true
            shift
            ;;
        -c|--compare)
            COMPARE_FILE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -m|--benchmem)
            BENCHMEM="-benchmem"
            shift
            ;;
        -t|--benchtime)
            BENCHTIME="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -s, --service <name>     Run benchmarks for specific service only"
            echo "  -b, --baseline           Save as baseline for future comparison"
            echo "  -c, --compare <file>     Compare against previous baseline"
            echo "  -v, --verbose            Show verbose benchmark output"
            echo "  -m, --benchmem           Include memory allocation statistics"
            echo "  -t, --benchtime <time>   Run each benchmark for specified time (default: 1s)"
            echo ""
            echo "Services: crawler, source-manager, classifier, publisher, auth, search"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Run all benchmarks"
            echo "  $0 -s crawler -m                      # Run crawler benchmarks with memory stats"
            echo "  $0 -b                                 # Run all and save as baseline"
            echo "  $0 -c baseline_20260104.txt           # Compare against baseline"
            echo "  $0 -s publisher -t 5s -m              # Run publisher for 5s with memory"
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
if [ -n "$SPECIFIC_SERVICE" ] && [ -z "${SERVICE_DIRS[$SPECIFIC_SERVICE]}" ]; then
    echo -e "${RED}Error: Invalid service '$SPECIFIC_SERVICE'${NC}"
    echo "Valid services: ${!SERVICE_DIRS[@]}"
    exit 1
fi

# Create baseline directory
mkdir -p "$BASELINE_DIR"

# Determine which services to run
if [ -n "$SPECIFIC_SERVICE" ]; then
    SERVICES=("$SPECIFIC_SERVICE")
else
    SERVICES=("${!SERVICE_DIRS[@]}")
fi

echo -e "${BLUE}=== North Cloud Benchmark Suite ===${NC}"
echo "Timestamp: $(date)"
echo "Services: ${SERVICES[*]}"
echo "Benchtime: $BENCHTIME"
[ -n "$BENCHMEM" ] && echo "Memory stats: enabled"
[ -n "$VERBOSE" ] && echo "Verbose: enabled"
echo ""

# Output file for results
if [ "$SAVE_BASELINE" = true ]; then
    OUTPUT_FILE="${BASELINE_DIR}/baseline_${TIMESTAMP}.txt"
    echo -e "${YELLOW}Saving baseline to: $OUTPUT_FILE${NC}"
    echo ""
else
    OUTPUT_FILE="${BASELINE_DIR}/results_${TIMESTAMP}.txt"
fi

# Write header to output file
{
    echo "# North Cloud Benchmark Results"
    echo "# Timestamp: $(date)"
    echo "# Services: ${SERVICES[*]}"
    echo "# Benchtime: $BENCHTIME"
    echo ""
} > "$OUTPUT_FILE"

# Run benchmarks for each service
TOTAL_BENCHMARKS=0
FAILED_SERVICES=()

for service in "${SERVICES[@]}"; do
    service_dir="${SERVICE_DIRS[$service]}"

    echo -e "${GREEN}Running benchmarks for ${service}...${NC}"

    if [ ! -d "$service_dir" ]; then
        echo -e "${RED}✗ Service directory not found: $service_dir${NC}"
        FAILED_SERVICES+=("$service")
        continue
    fi

    cd "$service_dir"

    # Count benchmarks
    BENCH_COUNT=$(go test -list 'Benchmark.*' ./... 2>/dev/null | grep -c '^Benchmark' || echo "0")

    if [ "$BENCH_COUNT" -eq 0 ]; then
        echo -e "${YELLOW}⚠ No benchmarks found in $service${NC}"
        cd - > /dev/null
        continue
    fi

    echo "Found $BENCH_COUNT benchmarks"

    # Run benchmarks
    {
        echo "## $service ($BENCH_COUNT benchmarks)"
        echo ""
    } >> "../$OUTPUT_FILE"

    if go test -bench=. -benchtime="$BENCHTIME" $BENCHMEM $VERBOSE ./... 2>&1 | tee -a "../$OUTPUT_FILE"; then
        echo -e "${GREEN}✓ $service benchmarks completed${NC}"
        TOTAL_BENCHMARKS=$((TOTAL_BENCHMARKS + BENCH_COUNT))
    else
        echo -e "${RED}✗ $service benchmarks failed${NC}"
        FAILED_SERVICES+=("$service")
    fi

    echo "" >> "../$OUTPUT_FILE"
    echo ""

    cd - > /dev/null
done

# Summary
echo -e "${BLUE}=== Benchmark Summary ===${NC}"
echo "Total benchmarks run: $TOTAL_BENCHMARKS"
echo "Results saved to: $OUTPUT_FILE"

if [ ${#FAILED_SERVICES[@]} -gt 0 ]; then
    echo -e "${RED}Failed services: ${FAILED_SERVICES[*]}${NC}"
fi

# Compare with baseline if requested
if [ -n "$COMPARE_FILE" ]; then
    if [ ! -f "$COMPARE_FILE" ]; then
        echo -e "${RED}Error: Baseline file not found: $COMPARE_FILE${NC}"
        exit 1
    fi

    echo ""
    echo -e "${BLUE}=== Benchmark Comparison ===${NC}"
    echo "Comparing against: $COMPARE_FILE"
    echo ""

    # Use benchstat if available
    if command -v benchstat &> /dev/null; then
        benchstat "$COMPARE_FILE" "$OUTPUT_FILE"
    else
        echo -e "${YELLOW}Note: Install benchstat for detailed comparison:${NC}"
        echo "  go install golang.org/x/perf/cmd/benchstat@latest"
        echo ""
        echo "Manual comparison:"
        echo "  Baseline: $COMPARE_FILE"
        echo "  Current:  $OUTPUT_FILE"
    fi
fi

if [ "$SAVE_BASELINE" = true ]; then
    echo ""
    echo -e "${GREEN}✓ Baseline saved successfully${NC}"
    echo "Use this baseline for comparison:"
    echo "  $0 -c $OUTPUT_FILE"
fi

echo ""
echo -e "${BLUE}Done!${NC}"

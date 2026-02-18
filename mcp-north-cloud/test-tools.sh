#!/bin/bash

# Test script for MCP North Cloud Server
# Tests tool registration based on MCP_ENV (local=18, prod=24)

set -e

MCP_BIN="./bin/mcp-north-cloud"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to send a JSON-RPC request
send_request() {
    local tool_name=$1
    local args=$2

    echo -e "${YELLOW}Testing: $tool_name${NC}"

    # Create JSON-RPC request
    request=$(cat <<EOF
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "$tool_name",
    "arguments": $args
  }
}
EOF
)

    # Send request and capture response
    response=$(echo "$request" | $MCP_BIN)

    # Check if response contains error
    if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo -e "${RED}✗ FAILED: $tool_name${NC}"
        echo "$response" | jq '.error'
        return 1
    else
        echo -e "${GREEN}✓ PASSED: $tool_name${NC}"
        return 0
    fi
}

# Function to test initialize
test_initialize() {
    echo -e "${YELLOW}Testing: initialize${NC}"
    request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
    response=$(timeout 10 bash -c "echo '$request' | $MCP_BIN 2>/dev/null" | head -1)
    if echo "$response" | jq -e '.result.protocolVersion' > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED: initialize${NC}"
        return 0
    else
        echo -e "${RED}✗ FAILED: initialize${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Function to test tools/list
test_tools_list() {
    echo -e "${YELLOW}Testing: tools/list${NC}"
    request='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
    response=$(timeout 10 bash -c "echo '$request' | $MCP_BIN 2>/dev/null" | head -1)

    # Count tools
    tool_count=$(echo "$response" | jq '.result.tools | length' 2>/dev/null)

    if [ "${MCP_ENV:-local}" = "prod" ]; then
        expected_tools=24
    else
        expected_tools=18
    fi

    if [ "$tool_count" -eq "$expected_tools" ]; then
        echo -e "${GREEN}✓ PASSED: tools/list (found $tool_count tools, expected $expected_tools for ${MCP_ENV:-local})${NC}"
        return 0
    else
        echo -e "${RED}✗ FAILED: tools/list (expected $expected_tools tools for ${MCP_ENV:-local}, found $tool_count)${NC}"
        echo "Response: $response"
        return 1
    fi
}

echo "======================================"
echo "MCP North Cloud Server Tool Tests"
echo "======================================"
echo ""

# Check if binary exists
if [ ! -f "$MCP_BIN" ]; then
    echo -e "${RED}Error: Binary not found at $MCP_BIN${NC}"
    echo "Please build the binary first: go build -o bin/mcp-north-cloud main.go"
    exit 1
fi

# Set environment variables for testing
export INDEX_MANAGER_URL="http://localhost:8090"
export CRAWLER_URL="http://localhost:8060"
export SOURCE_MANAGER_URL="http://localhost:8050"
export PUBLISHER_URL="http://localhost:8070"
export SEARCH_URL="http://localhost:8090"
export CLASSIFIER_URL="http://localhost:8070"

passed=0
failed=0

# Test protocol methods
test_initialize && ((passed++)) || ((failed++))
test_tools_list && ((passed++)) || ((failed++))

# Optional: test prompts and resources when MCP_TEST_PROMPTS=1
if [ "${MCP_TEST_PROMPTS:-0}" = "1" ]; then
    # prompts/list
    echo -e "${YELLOW}Testing: prompts/list${NC}"
    req='{"jsonrpc":"2.0","id":1,"method":"prompts/list","params":{}}'
    resp=$(timeout 5 bash -c "echo '$req' | $MCP_BIN" 2>&1 | head -1)
    n=$(echo "$resp" | jq '.result.prompts | length' 2>/dev/null || echo "0")
    if [ "${n:-0}" -ge 1 ]; then
        echo -e "${GREEN}✓ PASSED: prompts/list ($n prompts)${NC}"
        ((passed++)) || true
    else
        echo -e "${RED}✗ FAILED: prompts/list${NC}"
        ((failed++)) || true
    fi
    # resources/list
    echo -e "${YELLOW}Testing: resources/list${NC}"
    req='{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}'
    resp=$(timeout 5 bash -c "echo '$req' | $MCP_BIN" 2>&1 | head -1)
    n=$(echo "$resp" | jq '.result.resources | length' 2>/dev/null || echo "0")
    if [ "${n:-0}" -ge 1 ]; then
        echo -e "${GREEN}✓ PASSED: resources/list ($n resources)${NC}"
        ((passed++)) || true
    else
        echo -e "${RED}✗ FAILED: resources/list${NC}"
        ((failed++)) || true
    fi
fi

# Note: The following tool tests require actual services to be running
# For now, we'll just test that the tools are registered correctly

echo ""
echo "======================================"
echo "Tool Registration Tests Complete"
echo "======================================"
echo -e "Passed: ${GREEN}$passed${NC}"
echo -e "Failed: ${RED}$failed${NC}"
echo ""

if [ $failed -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
    echo "Environment: ${MCP_ENV:-local}"
    echo ""
    echo "Shared (15): onboard_source, list_crawl_jobs, get_crawl_stats, add_source,"
    echo "  list_sources, update_source, test_source, list_routes, list_channels,"
    echo "  preview_route, get_publish_history, get_publisher_stats, search_articles,"
    echo "  classify_article, list_indexes"
    if [ "${MCP_ENV:-local}" = "local" ]; then
        echo "Local-only (3): lint_file, build_service, test_service"
        echo "Total: 18 tools"
    else
        echo "Prod-only (9): start_crawl, schedule_crawl, control_crawl_job,"
        echo "  delete_source, create_route, create_channel, delete_route,"
        echo "  delete_index, get_auth_token"
        echo "Total: 24 tools"
    fi
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi

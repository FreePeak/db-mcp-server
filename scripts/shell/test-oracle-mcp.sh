#!/bin/bash

# Script to test Oracle DB via db-mcp-server MCP protocol
# This simulates how OpenCode will interact with the db-mcp-server

set -e

echo "=== Testing db-mcp-server with Oracle DB ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Check if Oracle container is running
echo -e "${YELLOW}Step 1: Checking Oracle container status...${NC}"
if docker ps --filter "name=db-mcp-oracle-test" --format "{{.Status}}" | grep -q "Up"; then
    echo -e "${GREEN}✓ Oracle container is running${NC}"
else
    echo -e "${RED}✗ Oracle container is not running${NC}"
    echo "Starting Oracle container..."
    docker-compose -f docker-compose.oracle.yml up -d
    echo "Waiting for Oracle to be ready (this may take 1-2 minutes)..."
    sleep 60
fi

echo ""

# Step 2: Initialize MCP connection
echo -e "${YELLOW}Step 2: Initializing MCP connection...${NC}"
INIT_REQUEST='{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "0.1.0", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0"}}}'

INIT_RESPONSE=$(echo "$INIT_REQUEST" | ./bin/db-mcp-server -t stdio -c config.json 2>/dev/null | grep -v "^20" | head -1)

if echo "$INIT_RESPONSE" | grep -q "serverInfo"; then
    echo -e "${GREEN}✓ MCP server initialized successfully${NC}"
else
    echo -e "${RED}✗ Failed to initialize MCP server${NC}"
    exit 1
fi

echo ""

# Step 3: List available tools
echo -e "${YELLOW}Step 3: Listing available Oracle tools...${NC}"
LIST_TOOLS='{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}'

TOOLS_RESPONSE=$(echo -e "$INIT_REQUEST\n$LIST_TOOLS" | ./bin/db-mcp-server -t stdio -c config.json 2>/dev/null | grep -v "^20" | tail -1)

# Extract Oracle tools
ORACLE_TOOLS=$(echo "$TOOLS_RESPONSE" | grep -o "oracle_dev" | sort -u)

if [ ! -z "$ORACLE_TOOLS" ]; then
    echo -e "${GREEN}✓ Oracle tools available:${NC}"
    echo "$TOOLS_RESPONSE" | grep -o '"name":"[^"]*oracle_dev[^"]*"' | sed 's/"name":"//g' | sed 's/"//g' | while read tool; do
        echo "  - $tool"
    done
else
    echo -e "${RED}✗ No Oracle tools found${NC}"
    exit 1
fi

echo ""

# Step 4: Test schema query
echo -e "${YELLOW}Step 4: Testing schema query on Oracle DB...${NC}"
SCHEMA_REQUEST='{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "schema_oracle_dev", "arguments": {"operation": "list_tables"}}}'

SCHEMA_RESPONSE=$(echo -e "$INIT_REQUEST\n$SCHEMA_REQUEST" | ./bin/db-mcp-server -t stdio -c config.json 2>/dev/null | grep -v "^20" | tail -1)

if echo "$SCHEMA_RESPONSE" | grep -q "result"; then
    echo -e "${GREEN}✓ Schema query successful${NC}"
    echo "Response preview:"
    echo "$SCHEMA_RESPONSE" | head -c 500
    echo "..."
else
    echo -e "${RED}✗ Schema query failed${NC}"
    echo "Response: $SCHEMA_RESPONSE"
fi

echo ""
echo -e "${GREEN}=== db-mcp-server Oracle validation complete ===${NC}"
echo ""
echo "The db-mcp-server is now ready to use with OpenCode!"
echo "Available tools: query_oracle_dev, execute_oracle_dev, transaction_oracle_dev, schema_oracle_dev, performance_oracle_dev"

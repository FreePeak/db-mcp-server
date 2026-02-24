#!/bin/bash

# Quick validation script - Run this once Oracle container is ready
# Usage: ./quick-validate-oracle.sh

set -e

echo "======================================"
echo "  Oracle + db-mcp-server Validation"
echo "======================================"
echo ""

# Check if Oracle container exists and is running
if docker ps --filter "name=db-mcp-oracle-test" --format "{{.Names}}" | grep -q "db-mcp-oracle-test"; then
    echo "✓ Oracle container is running"
    
    # Test db-mcp-server with Oracle
    echo ""
    echo "Testing db-mcp-server Oracle tools..."
    echo ""
    
    # Create a test MCP request
    cat > /tmp/mcp-test.json <<EOF
{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "0.1.0", "capabilities": {}, "clientInfo": {"name": "validator", "version": "1.0"}}}
{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
EOF

    # Run the test
    RESULT=$(cat /tmp/mcp-test.json | ./bin/db-mcp-server -t stdio -c config.oracle-only.json 2>/dev/null | grep -v "^20")
    
    # Check for Oracle tools
    if echo "$RESULT" | grep -q "oracle_dev"; then
        echo "✓ Oracle tools registered successfully"
        echo ""
        echo "Available Oracle tools:"
        echo "$RESULT" | grep -o '"name":"[^"]*oracle_dev[^"]*"' | sed 's/"name":"//g' | sed 's/"//g' | sed 's/^/  - /'
        echo ""
        echo "======================================"
        echo "  ✅ VALIDATION SUCCESSFUL"
        echo "======================================"
        echo ""
        echo "Next steps:"
        echo "1. Start OpenCode: opencode"
        echo "2. Test: use db-mcp-server to query oracle_dev"
        echo ""
    else
        echo "✗ Oracle tools not found"
        echo "Response: $RESULT"
        exit 1
    fi
    
else
    echo "✗ Oracle container not running"
    echo ""
    echo "Please wait for the Oracle image to finish downloading, then run:"
    echo "  docker-compose -f docker-compose.oracle.yml up -d"
    echo ""
    echo "Monitor progress with:"
    echo "  docker logs -f db-mcp-oracle-test"
    echo ""
    exit 1
fi

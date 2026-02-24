#!/bin/bash

echo "========================================="
echo "Oracle Database Test Script"
echo "========================================="
echo ""

# Test 1: Check if container is running
echo "1. Checking Oracle container status..."
if docker ps | grep -q "db-mcp-oracle-test"; then
    echo "   ✓ Container is running"
else
    echo "   ✗ Container is NOT running"
    exit 1
fi
echo ""

# Test 2: Check if port 1521 is accessible
echo "2. Checking if port 1521 is listening..."
if lsof -i :1521 > /dev/null 2>&1; then
    echo "   ✓ Port 1521 is listening"
else
    echo "   ✗ Port 1521 is NOT listening"
    exit 1
fi
echo ""

# Test 3: Test connection with testuser
echo "3. Testing database connection as testuser..."
RESULT=$(docker exec db-mcp-oracle-test bash -c "echo 'SELECT '\''Connection OK'\'' as status FROM dual;' | sqlplus -s testuser/testpass@localhost:1521/XEPDB1 2>&1" | grep "Connection OK")
if [ -n "$RESULT" ]; then
    echo "   ✓ Connection successful"
else
    echo "   ✗ Connection failed"
    exit 1
fi
echo ""

# Test 4: Query test data
echo "4. Querying test_users table..."
echo "   Running: SELECT * FROM test_users"
docker exec db-mcp-oracle-test bash -c "sqlplus -s testuser/testpass@localhost:1521/XEPDB1 <<'EOF'
SET LINESIZE 200
SET PAGESIZE 50
COLUMN id FORMAT 999
COLUMN username FORMAT A20
COLUMN email FORMAT A30
COLUMN created_date FORMAT A20

SELECT * FROM test_users ORDER BY id;
EXIT;
EOF"
echo ""

# Test 5: Show connection info
echo "5. Connection Information for db-mcp-server:"
echo "   ----------------------------------------"
echo "   ID:           oracle_dev"
echo "   Host:         localhost"
echo "   Port:         1521"
echo "   Service Name: XEPDB1"
echo "   User:         testuser"
echo "   Password:     testpass"
echo ""

echo "========================================="
echo "All tests passed! ✓"
echo "========================================="
echo ""
echo "Next steps:"
echo "1. Restart OpenCode to reload the MCP server"
echo "2. Test with: db-mcp-server_query_oracle_dev"
echo ""

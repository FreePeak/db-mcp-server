#!/bin/bash

# Oracle DB Testing Script for db-mcp-server
# This script will test the Oracle database once the container is ready

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================="
echo "  DB-MCP-SERVER ORACLE TEST SUITE"
echo "=========================================${NC}"
echo ""

# Function to check if Oracle container is ready
check_oracle_ready() {
    echo -e "${YELLOW}Checking Oracle container status...${NC}"
    
    if ! docker ps --filter "name=db-mcp-oracle-test" --format "{{.Status}}" | grep -q "Up"; then
        echo -e "${RED}✗ Oracle container is not running${NC}"
        echo ""
        echo "Please ensure:"
        echo "1. Oracle image is downloaded: docker images | grep oracle"
        echo "2. Start container: docker-compose -f docker-compose.oracle.yml up -d"
        echo "3. Wait for initialization: docker logs -f db-mcp-oracle-test"
        echo ""
        return 1
    fi
    
    echo -e "${GREEN}✓ Oracle container is running${NC}"
    
    # Check if database is ready
    if docker logs db-mcp-oracle-test 2>&1 | grep -q "DATABASE IS READY"; then
        echo -e "${GREEN}✓ Oracle database is ready${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ Oracle is starting up (this can take 1-2 minutes)${NC}"
        echo "Monitor with: docker logs -f db-mcp-oracle-test"
        return 1
    fi
}

# Check prerequisites
echo -e "${BLUE}=== Prerequisites Check ===${NC}"
echo ""

if ! check_oracle_ready; then
    exit 1
fi

echo ""
echo -e "${BLUE}=== Test 1: List Available Databases ===${NC}"
echo ""

# We'll use the db-mcp-server tools through MCP
echo "Available databases in db-mcp-server configuration:"
echo "  - oracle_dev (localhost:1521/XEPDB1)"
echo "  - oracle_cloud (cloud wallet)"
echo "  - oracle_rac (RAC cluster)"
echo ""

echo -e "${BLUE}=== Test 2: Test Oracle Connection ===${NC}"
echo ""

# Test basic connection with a simple query
echo "Testing connection to oracle_dev with SELECT FROM DUAL..."
echo ""

# Create test query file
cat > /tmp/oracle-test-query.sql << 'EOSQL'
SELECT 
    'Hello from Oracle!' as message,
    SYSDATE as current_time,
    USER as current_user,
    SYS_CONTEXT('USERENV', 'DB_NAME') as database_name
FROM DUAL
EOSQL

echo "Query:"
cat /tmp/oracle-test-query.sql
echo ""

echo -e "${BLUE}=== Test 3: Schema Exploration ===${NC}"
echo ""

echo "Getting Oracle schema information..."
echo ""

# List user tables
cat > /tmp/oracle-list-tables.sql << 'EOSQL'
SELECT table_name, num_rows, tablespace_name
FROM user_tables
ORDER BY table_name
EOSQL

echo "Query to list tables:"
cat /tmp/oracle-list-tables.sql
echo ""

echo -e "${BLUE}=== Test 4: System Views Query ===${NC}"
echo ""

# Query system info
cat > /tmp/oracle-system-info.sql << 'EOSQL'
SELECT 
    instance_name,
    version,
    status,
    database_status
FROM v$instance
EOSQL

echo "Query for system information:"
cat /tmp/oracle-system-info.sql
echo ""

echo -e "${BLUE}=== Test 5: Performance Test ===${NC}"
echo ""

# Create a test table
cat > /tmp/oracle-create-test.sql << 'EOSQL'
CREATE TABLE mcp_test (
    id NUMBER PRIMARY KEY,
    test_name VARCHAR2(100),
    test_value VARCHAR2(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
EOSQL

echo "Creating test table:"
cat /tmp/oracle-create-test.sql
echo ""

# Insert test data
cat > /tmp/oracle-insert-test.sql << 'EOSQL'
INSERT INTO mcp_test (id, test_name, test_value) 
VALUES (1, 'db-mcp-server-test', 'Testing Oracle connectivity via MCP protocol')
EOSQL

echo "Inserting test data:"
cat /tmp/oracle-insert-test.sql
echo ""

# Query test data
cat > /tmp/oracle-query-test.sql << 'EOSQL'
SELECT * FROM mcp_test ORDER BY id
EOSQL

echo "Querying test data:"
cat /tmp/oracle-query-test.sql
echo ""

echo -e "${GREEN}========================================="
echo "  Test Script Prepared"
echo "=========================================${NC}"
echo ""
echo "To execute these tests via OpenCode, use:"
echo ""
echo "  1. List databases:"
echo "     use db-mcp-server list_databases tool"
echo ""
echo "  2. Test connection:"
echo "     use query_oracle_dev to run: SELECT 'Hello' FROM DUAL"
echo ""
echo "  3. Get schema:"
echo "     use schema_oracle_dev tool"
echo ""
echo "  4. Execute DDL:"
echo "     use execute_oracle_dev to create the test table"
echo ""
echo "  5. Insert data:"
echo "     use execute_oracle_dev to insert test data"
echo ""
echo "  6. Query data:"
echo "     use query_oracle_dev to select from mcp_test"
echo ""
echo -e "${YELLOW}Note: Make sure Oracle container is fully initialized before running queries${NC}"
echo ""

# Cleanup test files
# rm -f /tmp/oracle-*.sql

echo "Test SQL files created in /tmp/oracle-*.sql for reference"
echo ""

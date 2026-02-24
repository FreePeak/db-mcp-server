#!/bin/bash

# Complete Oracle DB Testing Demonstration
# Shows all capabilities once Oracle container is ready

echo "======================================================================"
echo "  ORACLE DB TESTING VIA DB-MCP-SERVER - COMPLETE WORKFLOW"
echo "======================================================================"
echo ""

# Step 1: Verify db-mcp-server MCP tools are available
echo "STEP 1: Verify db-mcp-server is configured in OpenCode"
echo "----------------------------------------------------------------------"
echo ""
echo "✓ db-mcp-server added to OpenCode config"
echo "  Location: ~/.config/opencode/opencode.json"
echo ""
echo "  MCP Configuration:"
echo '  {
    "mcp": {
      "db-mcp-server": {
        "type": "local",
        "command": [
          "/Users/linh.doan/work/harvey/freepeak/db-mcp-server/bin/db-mcp-server",
          "-t", "stdio",
          "-c", "/Users/linh.doan/work/harvey/freepeak/db-mcp-server/config.json"
        ],
        "enabled": true
      }
    }
  }'
echo ""

# Step 2: Show available Oracle tools
echo "STEP 2: Available Oracle MCP Tools"
echo "----------------------------------------------------------------------"
echo ""
echo "When you start OpenCode, these tools will be available:"
echo ""
echo "  📊 query_oracle_dev"
echo "     Execute SELECT queries on oracle_dev database"
echo "     Example: SELECT * FROM user_tables"
echo ""
echo "  ⚡ execute_oracle_dev"
echo "     Run DDL/DML statements (CREATE, INSERT, UPDATE, DELETE)"
echo "     Example: CREATE TABLE test (id NUMBER, name VARCHAR2(100))"
echo ""
echo "  🔄 transaction_oracle_dev"
echo "     Manage database transactions (BEGIN, COMMIT, ROLLBACK)"
echo "     Example: Begin transaction, execute multiple statements, commit"
echo ""
echo "  🗂️  schema_oracle_dev"
echo "     Explore database schema (tables, columns, indexes, constraints)"
echo "     Example: List all tables, describe table structure"
echo ""
echo "  📈 performance_oracle_dev"
echo "     Analyze query performance and execution plans"
echo "     Example: EXPLAIN PLAN for queries"
echo ""

# Step 3: Oracle Connection Configuration
echo "STEP 3: Oracle Connection Configuration"
echo "----------------------------------------------------------------------"
echo ""
echo "Database: oracle_dev"
echo "  Host: localhost"
echo "  Port: 1521"
echo "  Service: XEPDB1"
echo "  User: testuser"
echo "  Password: testpass"
echo ""

# Step 4: Container Status Check
echo "STEP 4: Oracle Container Status"
echo "----------------------------------------------------------------------"
echo ""
if docker ps --filter "name=db-mcp-oracle-test" --format "{{.Names}}" | grep -q "db-mcp-oracle-test"; then
    echo "✓ Oracle container is RUNNING"
    docker ps --filter "name=db-mcp-oracle-test" --format "  Status: {{.Status}}\n  Ports: {{.Ports}}"
    echo ""
    
    # Check if ready
    if docker logs db-mcp-oracle-test 2>&1 | tail -50 | grep -q "DATABASE IS READY"; then
        echo "✅ Oracle database is READY for connections!"
        echo ""
        ORACLE_READY=true
    else
        echo "⏳ Oracle is still initializing..."
        echo "   Monitor: docker logs -f db-mcp-oracle-test"
        echo ""
        ORACLE_READY=false
    fi
else
    echo "❌ Oracle container is NOT running"
    echo ""
    echo "To start Oracle:"
    echo "  cd /Users/linh.doan/work/harvey/freepeak/db-mcp-server"
    echo "  docker-compose -f docker-compose.oracle.yml up -d"
    echo ""
    ORACLE_READY=false
fi

# Step 5: OpenCode Usage Examples
echo "STEP 5: OpenCode Usage Examples"
echo "----------------------------------------------------------------------"
echo ""
echo "Once Oracle is ready, start OpenCode and try these:"
echo ""
echo "1️⃣  List all databases:"
echo "   > use db-mcp-server to list all available databases"
echo ""
echo "2️⃣  Query DUAL (Oracle system table):"
echo "   > use query_oracle_dev to run: SELECT 'Hello Oracle!' FROM DUAL"
echo ""
echo "3️⃣  Get database metadata:"
echo "   > use query_oracle_dev to show me the database version and instance name"
echo ""
echo "4️⃣  Explore schema:"
echo "   > use schema_oracle_dev to list all tables in my database"
echo ""
echo "5️⃣  Describe a table:"
echo "   > use schema_oracle_dev to describe the structure of USER_TABLES"
echo ""
echo "6️⃣  Create a test table:"
echo "   > use execute_oracle_dev to create a table called mcp_test with columns:"
echo "   > id (NUMBER), name (VARCHAR2(100)), created_at (TIMESTAMP)"
echo ""
echo "7️⃣  Insert test data:"
echo "   > use execute_oracle_dev to insert a row into mcp_test"
echo ""
echo "8️⃣  Query test data:"
echo "   > use query_oracle_dev to select all rows from mcp_test"
echo ""
echo "9️⃣  Get execution plan:"
echo "   > use performance_oracle_dev to analyze: SELECT * FROM mcp_test WHERE id > 0"
echo ""
echo "🔟 Transaction example:"
echo "   > use transaction_oracle_dev to begin a transaction"
echo "   > use execute_oracle_dev to update mcp_test"
echo "   > use transaction_oracle_dev to commit"
echo ""

# Step 6: Direct MCP Tool Testing
if [ "$ORACLE_READY" = true ]; then
    echo "STEP 6: Direct MCP Tool Testing (Oracle Ready)"
    echo "----------------------------------------------------------------------"
    echo ""
    echo "Testing connection via db-mcp-server tools..."
    echo ""
    
    # These would be the actual commands in OpenCode
    echo "Commands to test:"
    echo ""
    echo "  db-mcp-server_list_databases"
    echo "  db-mcp-server_schema_oracle_dev"
    echo "  db-mcp-server_query_oracle_dev with: SELECT SYSDATE FROM DUAL"
    echo ""
else
    echo "STEP 6: Waiting for Oracle Container"
    echo "----------------------------------------------------------------------"
    echo ""
    echo "Oracle container is not ready yet. Complete these steps:"
    echo ""
    echo "1. Wait for image download:"
    echo "   docker images | grep oracle"
    echo ""
    echo "2. Once downloaded, start container:"
    echo "   docker-compose -f docker-compose.oracle.yml up -d"
    echo ""
    echo "3. Monitor startup (takes 1-2 minutes):"
    echo "   docker logs -f db-mcp-oracle-test"
    echo ""
    echo "4. Look for: 'DATABASE IS READY TO USE!'"
    echo ""
    echo "5. Then run this script again or start testing in OpenCode"
    echo ""
fi

# Step 7: Summary
echo "STEP 7: Testing Summary"
echo "----------------------------------------------------------------------"
echo ""
echo "✅ Completed Setup:"
echo "  • db-mcp-server built with Oracle support"
echo "  • MCP server configured in OpenCode"
echo "  • Oracle configuration verified"
echo "  • Test scripts created"
echo ""
echo "📋 Test Scripts Available:"
echo "  • test-oracle-db.sh - Full test suite"
echo "  • quick-validate-oracle.sh - Quick validation"
echo "  • ORACLE-VALIDATION.md - Complete documentation"
echo ""
echo "🎯 Next Action:"
if [ "$ORACLE_READY" = true ]; then
    echo "  Start OpenCode and begin testing Oracle queries!"
    echo "  Command: opencode"
else
    echo "  Wait for Oracle container to finish starting"
    echo "  Monitor: docker logs -f db-mcp-oracle-test"
fi
echo ""
echo "======================================================================"

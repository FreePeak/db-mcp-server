# Oracle DB Validation with db-mcp-server on OpenCode

## Current Status

✅ **Completed:**
1. Built db-mcp-server binary with Oracle support (`-tags oracle`)
2. Added db-mcp-server to OpenCode config at `~/.config/opencode/opencode.json`
3. Verified MCP server can register Oracle tools:
   - query_oracle_dev
   - execute_oracle_dev
   - transaction_oracle_dev
   - schema_oracle_dev
   - performance_oracle_dev

⏳ **In Progress:**
- Oracle Docker image download (gvenzl/oracle-xe:21-slim ~2.5GB)

## Manual Validation Steps

### Step 1: Wait for Oracle Image Download

Check download status:
```bash
docker images | grep oracle
```

If not yet downloaded, manually pull:
```bash
docker pull gvenzl/oracle-xe:21-slim
```

### Step 2: Start Oracle Container

```bash
cd /Users/linh.doan/work/harvey/freepeak/db-mcp-server
docker-compose -f docker-compose.oracle.yml up -d
```

Wait for Oracle to initialize (1-2 minutes):
```bash
docker logs -f db-mcp-oracle-test
```

Look for: `DATABASE IS READY TO USE!`

### Step 3: Verify Container is Running

```bash
docker ps --filter "name=db-mcp-oracle-test"
```

Expected output:
```
CONTAINER ID   IMAGE                       STATUS         PORTS
xxx            gvenzl/oracle-xe:21-slim    Up X minutes   0.0.0.0:1521->1521/tcp
```

### Step 4: Test db-mcp-server Connection

Run the automated test script:
```bash
./test-oracle-mcp.sh
```

Or manually test:
```bash
# Initialize and list tools
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "0.1.0", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}}' | \
./bin/db-mcp-server -t stdio -c config.oracle-only.json 2>&1 | grep oracle_dev
```

Expected: You should see oracle_dev tools listed

### Step 5: Validate in OpenCode

1. Start OpenCode in this directory:
```bash
opencode
```

2. Test the db-mcp-server MCP integration:
```
use db-mcp-server to list all tables in the oracle_dev database
```

3. Query Oracle schema:
```
use the schema_oracle_dev tool to show me the database structure
```

4. Test a simple query:
```
use query_oracle_dev to run: SELECT * FROM dual
```

## Configuration Files

### OpenCode MCP Config
Location: `~/.config/opencode/opencode.json`

```json
{
  "mcp": {
    "db-mcp-server": {
      "type": "local",
      "command": [
        "/Users/linh.doan/work/harvey/freepeak/db-mcp-server/bin/db-mcp-server",
        "-t",
        "stdio",
        "-c",
        "/Users/linh.doan/work/harvey/freepeak/db-mcp-server/config.json"
      ],
      "enabled": true
    }
  }
}
```

### Oracle Connection Config
Location: `config.json` (oracle_dev section)

```json
{
  "id": "oracle_dev",
  "type": "oracle",
  "host": "localhost",
  "port": 1521,
  "service_name": "XEPDB1",
  "user": "testuser",
  "password": "testpass"
}
```

## Troubleshooting

### Issue: "unsupported database type: oracle"
**Solution:** Binary needs Oracle support. Rebuild with:
```bash
go build -tags oracle -o bin/db-mcp-server cmd/server/main.go
```

### Issue: "connection refused" or "ORA-12541"
**Solution:** Oracle container not ready. Wait longer or check logs:
```bash
docker logs db-mcp-oracle-test
```

### Issue: "ORA-01017: invalid username/password"
**Solution:** Wrong credentials. Use:
- User: `testuser`
- Password: `testpass`
- Service: `XEPDB1`

### Issue: Tools not showing in OpenCode
**Solution:** 
1. Restart OpenCode
2. Check MCP server logs: `logs/mcp-logger-*.log`
3. Verify config: `cat ~/.config/opencode/opencode.json | grep -A 10 mcp`

## Available Oracle Tools

Once validated, these tools will be available in OpenCode:

| Tool Name | Description |
|-----------|-------------|
| `query_oracle_dev` | Execute SELECT queries and retrieve data |
| `execute_oracle_dev` | Run INSERT, UPDATE, DELETE, DDL statements |
| `transaction_oracle_dev` | Manage BEGIN, COMMIT, ROLLBACK transactions |
| `schema_oracle_dev` | Explore tables, columns, indexes, constraints |
| `performance_oracle_dev` | Analyze query plans and performance metrics |

## Example OpenCode Queries

```
# List all tables
use schema_oracle_dev to list all tables

# Show table structure
use schema_oracle_dev to describe the EMPLOYEES table

# Run a query
use query_oracle_dev to select the first 10 rows from EMPLOYEES

# Get query execution plan
use performance_oracle_dev to explain the query: SELECT * FROM EMPLOYEES WHERE salary > 50000

# Create a table
use execute_oracle_dev to create a table: CREATE TABLE test_table (id NUMBER, name VARCHAR2(100))
```

## Success Criteria

✅ Oracle container running and healthy
✅ db-mcp-server connects to Oracle without errors
✅ Oracle tools registered in MCP server
✅ OpenCode can query Oracle through db-mcp-server
✅ Schema exploration works
✅ Query execution returns results

## Next Steps After Validation

1. Test with real queries on your Oracle database
2. Explore schema and data using OpenCode
3. Use AI to write and optimize SQL queries
4. Analyze query performance
5. Generate documentation from database schema

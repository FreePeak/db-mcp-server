# Oracle Database Docker Setup

## Quick Start

### 1. Start Oracle Database

```bash
docker-compose -f docker-compose.oracle.yml up -d
```

This will:
- Pull the `gvenzl/oracle-xe:21-slim` image
- Start Oracle XE 21c database
- Create a user `testuser` with password `testpass`
- Initialize test schema with `test_users` table
- Expose port `1521` on localhost

### 2. Wait for Database Initialization

Oracle database takes 1-2 minutes to fully initialize. Check the status:

```bash
docker-compose -f docker-compose.oracle.yml logs -f
```

Wait until you see: `DATABASE IS READY TO USE!`

### 3. Verify Connection

The `oracle_dev` connection in `config.json` is already configured:

```json
{
    "id": "oracle_dev",
    "type": "oracle",
    "host": "localhost",
    "port": 1521,
    "service_name": "XEPDB1",
    "user": "testuser",
    "password": "testpass",
    "max_open_conns": 50,
    "max_idle_conns": 10,
    "conn_max_lifetime_seconds": 1800,
    "query_timeout": 60
}
```

### 4. Test with db-mcp-server

Use any of the Oracle database tools available:

```bash
# List databases
db-mcp-server_list_databases

# Get schema
db-mcp-server_schema_oracle_dev

# Query data
db-mcp-server_query_oracle_dev
```

## Database Details

| Property | Value |
|----------|-------|
| Container Name | `db-mcp-oracle-test` |
| Image | `gvenzl/oracle-xe:21-slim` |
| Port | `1521` |
| Service Name | `XEPDB1` |
| System Password | `oracle` |
| App User | `testuser` |
| App Password | `testpass` |

## Available Connections

Your `config.json` has 3 Oracle connections configured:

1. **oracle_dev** - Local Docker Oracle XE (ready to use)
2. **oracle_cloud** - Oracle Cloud with wallet (needs wallet path and password)
3. **oracle_rac** - Oracle RAC cluster (needs host and credentials)

## Sample Data

The initialization script creates a `test_users` table with 3 sample records:

```sql
SELECT * FROM test_users;
```

## Stopping the Database

```bash
docker-compose -f docker-compose.oracle.yml down
```

To remove data volumes as well:

```bash
docker-compose -f docker-compose.oracle.yml down -v
```

## Troubleshooting

### Check if container is running
```bash
docker ps | grep oracle
```

### Check logs
```bash
docker logs db-mcp-oracle-test
```

### Connect with SQL*Plus
```bash
docker exec -it db-mcp-oracle-test sqlplus testuser/testpass@XEPDB1
```

### Reset database
```bash
docker-compose -f docker-compose.oracle.yml down -v
docker-compose -f docker-compose.oracle.yml up -d
```

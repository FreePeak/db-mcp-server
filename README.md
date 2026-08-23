<div align="center">

<img src="assets/logo.svg" alt="DB MCP Server Logo" width="300" />

# Multi Database MCP Server

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/FreePeak/db-mcp-server)](https://goreportcard.com/report/github.com/FreePeak/db-mcp-server)
[![Go Reference](https://pkg.go.dev/badge/github.com/FreePeak/db-mcp-server.svg)](https://pkg.go.dev/github.com/FreePeak/db-mcp-server)
[![Contributors](https://img.shields.io/github/contributors/FreePeak/db-mcp-server)](https://github.com/FreePeak/db-mcp-server/graphs/contributors)

<h3>A powerful multi-database server implementing the Model Context Protocol (MCP) to provide AI assistants with structured access to databases.</h3>

<div class="toc">
  <a href="#overview">Overview</a> •
  <a href="#core-concepts">Core Concepts</a> •
  <a href="#features">Features</a> •
  <a href="#supported-databases">Supported Databases</a> •
  <a href="#deployment-options">Deployment Options</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#available-tools">Available Tools</a> •
  <a href="#examples">Examples</a> •
  <a href="#troubleshooting">Troubleshooting</a> •
  <a href="#contributing">Contributing</a>
</div>

</div>

## Overview

The DB MCP Server provides a standardized way for AI models to interact with multiple databases simultaneously. Built on the [FreePeak/cortex](https://github.com/FreePeak/cortex) framework, it enables AI assistants to execute SQL queries, manage transactions, explore schemas, and analyze performance across different database systems through a unified interface.

## Core Concepts

### Multi-Database Support

Unlike traditional database connectors, DB MCP Server can connect to and interact with multiple databases concurrently:

```json
{
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "localhost",
      "port": 3306,
      "name": "db1",
      "user": "user1",
      "password": "password1"
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "localhost",
      "port": 5432,
      "name": "db2",
      "user": "user2",
      "password": "password2"
    },
    {
      "id": "oracle1",
      "type": "oracle",
      "host": "localhost",
      "port": 1521,
      "service_name": "XEPDB1",
      "user": "user3",
      "password": "password3"
    }
  ]
}
```

### Dynamic Tool Generation

For each connected database, the server automatically generates specialized tools:

```go
// For a database with ID "mysql1", these tools are generated:
query_mysql1       // Execute SQL queries
execute_mysql1     // Run data modification statements
transaction_mysql1 // Manage transactions
schema_mysql1      // Explore database schema
performance_mysql1 // Analyze query performance
```

### Clean Architecture

The server follows Clean Architecture principles with these layers:

1. **Domain Layer**: Core business entities and interfaces
2. **Repository Layer**: Data access implementations
3. **Use Case Layer**: Application business logic
4. **Delivery Layer**: External interfaces (MCP tools)

## Features

- **Simultaneous Multi-Database Support**: Connect to multiple MySQL, PostgreSQL, SQLite, and Oracle databases concurrently
- **Lazy Loading Mode**: Defer connection establishment until first use - perfect for setups with 10+ databases (enable with `--lazy-loading` flag)
- **Database-Specific Tool Generation**: Auto-creates specialized tools for each connected database
- **Clean Architecture**: Modular design with clear separation of concerns
- **OpenAI Agents SDK Compatibility**: Full compatibility for seamless AI assistant integration
- **Dynamic Database Tools**: Execute queries, run statements, manage transactions, explore schemas, analyze performance
- **Unified Interface**: Consistent interaction patterns across different database types
- **Connection Management**: Simple configuration for multiple database connections
- **Health Check**: Automatic validation of database connectivity on startup
- **Production Guardrails**: Per-database `read_only` enforcement (blocks writes through both `query_*` and `execute_*` tools), `max_rows` result truncation with explicit notices, and per-query timeouts

### Production Guardrails

Protect agent sessions against runaway queries and accidental writes:

| Setting | Scope | Effect |
|---------|-------|--------|
| `"read_only": true` | per database | Blocks write statements (`INSERT`, `UPDATE`, `DELETE`, DDL, data-modifying CTEs, stacked writes) through **both** query and execute tools, **and** enforces rejection at the database engine itself on PostgreSQL/TimescaleDB (`default_transaction_read_only=on`) and MySQL (`transaction_read_only=1`); SQLite opens `mode=ro`. Classification strips comments and string literals and defaults to deny for unrecognized statements. |
| `"max_rows": 1000` | per database | Truncates result sets at N rows with an explicit `[Truncated]` notice so the model knows to refine its query instead of losing context. `0` (default) means unlimited. When set, unbounded SELECTs also get a server-side row bound injected (engines stop early instead of materializing everything; existing limits respected) — see Auto-LIMIT below. |
| `"mask_pii": true` | per database | **Server-enforced PII masking**: query results have emails, phone numbers, credit cards, SSNs, IP addresses and long numeric identifiers redacted (`[EMAIL]`, `[PHONE]`, …) regardless of per-request parameters. Agents can also opt in per query with `"mask_pii": true`. |

#### Auto-LIMIT

With `max_rows` configured, read queries without their own top-level row
bound get one injected before execution:

- `SELECT * FROM users` → `SELECT * FROM users LIMIT 1000`
- Oracle gets a ROWNUM wrap instead (works on all versions, WITH-clause safe):
  `SELECT name FROM accounts` → `SELECT * FROM (SELECT name FROM accounts) WHERE ROWNUM <= 1000`
- Queries with an existing top-level `LIMIT` (or top-level `ROWNUM` / `FETCH FIRST` on Oracle) pass through untouched
- A subquery's `LIMIT` does **not** suppress injection (it never bounds the outer result)
- Non-SELECT statements are never modified

This bounds engine work, not just the context window.
| `"query_timeout": 30` | per database | Cancels queries that exceed the timeout in seconds. |

> **Defense in depth**: read-only is enforced in three layers — application classifier, engine session defaults, and (recommended) least-privilege database users. Oracle currently relies on the classifier plus user privileges.

## Supported Databases

| Database   | Status                    | Features                                                     |
| ---------- | ------------------------- | ------------------------------------------------------------ |
| MySQL      | ✅ Full Support           | Queries, Transactions, Schema Analysis, Performance Insights |
| PostgreSQL | ✅ Full Support (v9.6-17) | Queries, Transactions, Schema Analysis, Performance Insights |
| SQLite     | ✅ Full Support           | File-based & In-memory databases, SQLCipher encryption support |
| Oracle     | ✅ Full Support (10g-23c) | Queries, Transactions, Schema Analysis, RAC, Cloud Wallet, TNS |
| TimescaleDB| ✅ Full Support           | Hypertables, Time-Series Queries, Continuous Aggregates, Compression, Retention Policies |

## Deployment Options

The DB MCP Server can be deployed in multiple ways to suit different environments and integration needs:

### Docker Deployment

```bash
# Pull the latest image
docker pull freepeak/db-mcp-server:latest

# Run with mounted config file
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  -e TRANSPORT_MODE=sse \
  -e CONFIG_PATH=/app/my-config.json \
  -e DB_MCP_API_KEY=replace-me-with-a-long-random-string \
  freepeak/db-mcp-server
```

> **Note**: Mount to `/app/my-config.json` as the container has a default file at `/app/config.json`.

#### API-Key Authentication

The SSE and streamable-HTTP transports accept an `Authorization: Bearer <key>`
header. Set `DB_MCP_API_KEY` (or pass `-api-key`) when launching the Docker
container; clients must then send the matching bearer token on every request:

```bash
curl -H "Authorization: Bearer replace-me-with-a-long-random-string" \
     http://localhost:9092/sse
```

When no API key is configured the transport remains open (single-user /
development use). The middleware lives in `internal/delivery/mcp.APIKeyAuth`
and is exported so you can compose it with your own reverse proxy if you
front the container with nginx, Caddy, or Traefik.

### STDIO Mode (IDE Integration)

```bash
# Run the server in STDIO mode
./bin/server -t stdio -c config.json
```

For Cursor IDE integration, add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "stdio-db-mcp-server": {
      "command": "/path/to/db-mcp-server/server",
      "args": ["-t", "stdio", "-c", "/path/to/config.json"]
    }
  }
}
```

### SSE Mode (Server-Sent Events)

```bash
# Default configuration (localhost:9092)
./bin/server -t sse -c config.json

# Custom host and port
./bin/server -t sse -host 0.0.0.0 -port 8080 -c config.json
```

Client connection endpoint: `http://localhost:9092/sse`

### Source Code Installation

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Build the server
make build

# Run the server
./bin/server -t sse -c config.json
```

## Configuration

### Database Configuration File

Create a `config.json` file with your database connections:

```json
{
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "mysql1",
      "port": 3306,
      "name": "db1",
      "user": "user1",
      "password": "password1",
      "query_timeout": 60,
      "max_open_conns": 20,
      "max_idle_conns": 5,
      "conn_max_lifetime_seconds": 300,
      "conn_max_idle_time_seconds": 60,
      "read_only": false,
      "max_rows": 1000
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "postgres1",
      "port": 5432,
      "name": "db1",
      "user": "user1",
      "password": "password1"
    },
    {
      "id": "sqlite_app",
      "type": "sqlite",
      "database_path": "./data/app.db",
      "journal_mode": "WAL",
      "cache_size": 2000,
      "read_only": false,
      "use_modernc_driver": true,
      "query_timeout": 30,
      "max_open_conns": 1,
      "max_idle_conns": 1
    },
    {
      "id": "sqlite_encrypted",
      "type": "sqlite",
      "database_path": "./data/secure.db",
      "encryption_key": "your-secret-key-here",
      "journal_mode": "WAL",
      "use_modernc_driver": false
    },
    {
      "id": "sqlite_memory",
      "type": "sqlite",
      "database_path": ":memory:",
      "cache_size": 1000,
      "use_modernc_driver": true
    }
  ]
}
```

### Command-Line Options

```bash
# Basic syntax
./bin/server -t <transport> -c <config-file>

# SSE transport options
./bin/server -t sse -host <hostname> -port <port> -c <config-file>

# Lazy loading mode (recommended for 10+ databases)
./bin/server -t stdio -c <config-file> --lazy-loading

# Customize log directory (useful for multi-project setups)
./bin/server -t stdio -c <config-file> -log-dir /tmp/db-mcp-logs

# Inline database configuration
./bin/server -t stdio -db-config '{"connections":[...]}'

# Environment variable configuration
export DB_CONFIG='{"connections":[...]}'
./bin/server -t stdio
```

**Available Flags:**
- `-t, -transport`: Transport mode (`stdio` or `sse`)
- `-c, -config`: Path to database configuration file
- `-p, -port`: Server port for SSE mode (default: 9092)
- `-h, -host`: Server host for SSE mode (default: localhost)
- `-log-level`: Log level (`debug`, `info`, `warn`, `error`)
- `-log-dir`: Directory for log files (default: `./logs` in current directory)
- `-db-config`: Inline JSON database configuration
- `-unified-tools`: Register unified tools with a `database` parameter instead of per-database tools
- `-lazy-loading`: Establish connections on first use (recommended for 10+ databases)
- `-masking-audit-log`: JSONL file path for durable PII-masking audit events (append mode; survives restarts)
- `-query-history-log`: JSONL file path for durable query-history events (append mode; every executed statement with duration/outcome)
- `-risk-warn-at`: Minimum post-execution advisory level — `low`, `medium`, `high` (default), `critical`

**Environment defaults:** `-masking-audit-log`, `-query-history-log`, and `-risk-warn-at` can also be set declaratively via `DB_MCP_MASKING_AUDIT_LOG`, `DB_MCP_QUERY_HISTORY_LOG`, and `DB_MCP_RISK_WARN_AT`; explicit flags win over env values.

## Testing Without Docker (Free Cloud Databases)

The regression suite runs against free-tier managed cloud databases — no local containers needed:

```bash
# 1. Get a free database (no credit card): neon.tech, supabase.com, aiven.io, tidbcloud.com
# 2. Register it (live-ping validation through the server's own connection layer):
go run ./cmd/registerdb my_neon "postgresql://user:pass@ep-x.region.neon.tech/db?sslmode=require"

# Or skip registration entirely — env vars are auto-detected:
export NEON_DATABASE_URL="postgresql://..."   # also: SUPABASE_DATABASE_URL, AIVEN_DATABASE_URL,
                                              # TIDBCLOUD_DATABASE_URL, CLOUD_MYSQL_URL, DATABASE_URL

# 3. Run the cloud regression battery:
go test ./pkg/db/ -run TestCloudRegression -v
```

Providers are detected from hostnames automatically; sleeping free tiers (scale-to-zero) are woken by connect retry. With zero credentials configured, cloud tests skip gracefully.

## SQLite Configuration Options

When using SQLite databases, you can leverage these additional configuration options:

### SQLite Connection Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `database_path` | string | Required | Path to SQLite database file or `:memory:` for in-memory |
| `encryption_key` | string | - | Key for SQLCipher encrypted databases |
| `read_only` | boolean | false | Open database in read-only mode |
| `max_rows` | integer | unlimited | Maximum rows returned per query; larger results are truncated with an explicit notice. Works on all database types |
| `cache_size` | integer | 2000 | SQLite cache size in pages |
| `journal_mode` | string | "WAL" | Journal mode: DELETE, TRUNCATE, PERSIST, WAL, OFF |
| `use_modernc_driver` | boolean | true | Use modernc.org/sqlite (CGO-free) or mattn/go-sqlite3 |

### SQLite Examples

#### Basic File Database
```json
{
  "id": "my_sqlite_db",
  "type": "sqlite",
  "database_path": "./data/myapp.db",
  "journal_mode": "WAL",
  "cache_size": 2000
}
```

#### Encrypted Database (SQLCipher)
```json
{
  "id": "encrypted_db",
  "type": "sqlite",
  "database_path": "./data/secure.db",
  "encryption_key": "your-secret-encryption-key",
  "use_modernc_driver": false
}
```

#### In-Memory Database
```json
{
  "id": "memory_db",
  "type": "sqlite",
  "database_path": ":memory:",
  "cache_size": 1000
}
```

#### Read-Only Database
```json
{
  "id": "reference_data",
  "type": "sqlite",
  "database_path": "./data/reference.db",
  "read_only": true,
  "journal_mode": "DELETE"
}
```

## Oracle Configuration Options

When using Oracle databases, you can leverage these additional configuration options:

### Oracle Connection Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `host` | string | Required | Oracle database host |
| `port` | integer | 1521 | Oracle listener port |
| `service_name` | string | - | Service name (recommended for RAC) |
| `sid` | string | - | System identifier (legacy, use service_name instead) |
| `user` | string | Required | Database username |
| `password` | string | Required | Database password |
| `wallet_location` | string | - | Path to Oracle Cloud wallet directory |
| `tns_admin` | string | - | Path to directory containing tnsnames.ora |
| `tns_entry` | string | - | Named entry from tnsnames.ora |
| `edition` | string | - | Edition-Based Redefinition edition name |
| `pooling` | boolean | false | Enable driver-level connection pooling |
| `standby_sessions` | boolean | false | Allow queries on standby databases |
| `nls_lang` | string | AMERICAN_AMERICA.AL32UTF8 | Character set configuration |

### Oracle Examples

#### Basic Oracle Connection (Development)
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
  "conn_max_lifetime_seconds": 1800
}
```

#### Oracle with SID (Legacy)
```json
{
  "id": "oracle_legacy",
  "type": "oracle",
  "host": "oracledb.company.com",
  "port": 1521,
  "sid": "ORCL",
  "user": "app_user",
  "password": "app_password"
}
```

#### Oracle Cloud Autonomous Database (with Wallet)
```json
{
  "id": "oracle_cloud",
  "type": "oracle",
  "user": "ADMIN",
  "password": "your-cloud-password",
  "wallet_location": "/path/to/wallet_DBNAME",
  "service_name": "dbname_high"
}
```

#### Oracle RAC (Real Application Clusters)
```json
{
  "id": "oracle_rac",
  "type": "oracle",
  "host": "scan.company.com",
  "port": 1521,
  "service_name": "production",
  "user": "app_user",
  "password": "app_password",
  "max_open_conns": 100,
  "max_idle_conns": 20
}
```

#### Oracle with TNS Entry
```json
{
  "id": "oracle_tns",
  "type": "oracle",
  "tns_admin": "/opt/oracle/network/admin",
  "tns_entry": "PROD_DB",
  "user": "app_user",
  "password": "app_password"
}
```

#### Oracle with Edition-Based Redefinition
```json
{
  "id": "oracle_ebr",
  "type": "oracle",
  "host": "oracledb.company.com",
  "port": 1521,
  "service_name": "production",
  "user": "app_user",
  "password": "app_password",
  "edition": "v2_0"
}
```

### Oracle Connection String Priority

When multiple connection methods are configured, the following priority is used:

1. **TNS Entry** (if `tns_entry` and `tns_admin` are configured)
2. **Wallet** (if `wallet_location` is configured) - for Oracle Cloud
3. **Standard** (host:port/service_name) - default method

## Available Tools

For each connected database, DB MCP Server automatically generates these specialized tools:

### Query Tools

| Tool Name | Description |
|-----------|-------------|
| `query_<db_id>` | Execute SELECT queries and get results as a tabular dataset; `format: "csv"` / `"json"` / `"inserts"` return machine-readable output (RFC4180 CSV, array of row objects, or INSERT INTO DML for the queried table), honoring max_rows and PII masking; `count_only: true` returns the statement's row COUNT(*) without fetching rows; `timeout_ms` cancels the query past a per-call deadline; `page` + `page_size` window results and report the total matching rows in one call; `sample_rows: N` returns N randomly ordered rows with engine-aware ordering; `databases: "a,b"` fans the SELECT out over several databases with per-database sections (staging vs prod spot-check); `unused_indexes: true` lists barely-scanned indexes from engine usage stats (Postgres/MySQL, `min_scans` threshold, default 100); `long_queries: N` lists queries running longer than N seconds from the engine's activity catalog; `save_query: <name>` / `saved_queries: true` / `run_saved_query: <name>` bookmark and replay named SELECTs per database |
| `execute_<db_id>` | Run data manipulation statements (INSERT, UPDATE, DELETE) |
| `transaction_<db_id>` | Begin, commit, and rollback transactions |

### Schema Tools

| Tool Name | Description |
|-----------|-------------|
| `schema_<db_id>` | Get information about tables, columns, indexes, and foreign keys |
| `generate_schema_<db_id>` | Render every table as application code from the live schema: `format: "go"` (exported structs with `db` tags, initialism casing) or `"typescript"` (interfaces) |

### Performance Tools

| Tool Name | Description |
|-----------|-------------|
| `performance_<db_id>` | Analyze query performance and get optimization suggestions. Actions: `suggest_indexes` (alias-safe CREATE INDEX DDL from JOIN/WHERE/ORDER BY/GROUP BY columns vs live catalogs, composite-aware, PK-aware), `engine_slow_queries` (top statements from `pg_stat_statements` / MySQL digests), `list_sessions` (active sessions from pg_stat_activity / processlist), `lock_waits` (who blocks whom via pg_blocking_pids / sys.innodb_lock_waits), `long_transactions` with optional `min_age_secs` (idle-in-transaction sessions holding locks and blocking vacuum, oldest first), `replication_status` (attached replicas with replay position/lag via pg_stat_replication / SHOW REPLICA STATUS; empty = no replicas attached), `connection_saturation` (engine connections vs max_connections; ≥80% warning, ≥95% critical — "too many clients" incidents become visible before they fire), `cancel_query` with `session_id` (pg_cancel_backend / KILL QUERY), plus tracked metrics and static SQL issue suggestions |
| `explain_<db_id>` | Show the execution plan for a SQL statement without running it; `analyze: true` executes with timing/buffer stats (PostgreSQL/MySQL). Writes stay blocked on read-only databases |
| `describe_<db_id>` | Inspect one table's columns, indexes, row estimate, constraints (PK/FK/UNIQUE) with FK target resolution (`author_id -> authors(id)`) via engine catalog queries; pass `profile_column` for a one-column statistical profile (rows, null count, cardinality, min/max, top values), or `related_key` with a primary-key value to follow the row's foreign keys to parent rows and list referencing child rows, or `duplicates_column` to report repeated values with counts and an example PK per group, or `profile: true` for a whole-table per-column profile (rows, NULL count, distinct count, min/max) |
| `schema_<db_id>` | List tables/columns; `format: "mermaid"` renders the foreign-key graph as a Mermaid ER diagram; `format: "sensitive"` reports PII-suspect columns (name heuristics + content sampling) with masking guidance; `format: "compare"` diffs structure (tables, columns, indexes, constraints, and views), `format: "compare_data_counts"` row counts, and `format: "compare_samples"` row-level content (one table) against a `compare_with` database; `format: "views"` lists views, `format: "triggers"` triggers, `format: "routines"` stored functions/procedures, `format: "types"` user-defined enum/composite types, `format: "ddl"` verbatim CREATE statements (sqlite), `format: "orphans"` counts child rows violating each foreign key, `format: "sizes"` reports row counts and disk size per table; `format: "type_consistency"` flags shared column names with divergent types across tables (joins will coerce or fail), `format: "no_pk"` flags user tables lacking a PRIMARY KEY (replication breaks, rows unaddressable), `format: "checks"` lists CHECK-constraint clauses grouped by table (the business rules valid data must satisfy; Postgres/MySQL 8+), `format: "fk_indexes"` flags foreign-key child columns with no leading index (parent deletes scan the child table; candidate DDL included), `format: "redundant_indexes"` flags non-unique indexes whose column list is a prefix of a wider sibling plus exact-duplicate index pairs (write amplification, no read benefit), `format: "key_diff"` reports which primary-key values of one table exist only on each side of a `compare_with` database (copy/sync verification), `format: "grants"` lists table privileges grouped by grantee from the engine catalogs (Postgres/MySQL), `format: "sequences"` flags integer-key sequences at ≥80% of their ceiling (exhaustion is a silent insert-failure incident; Postgres), `format: "dependency_order"` renders the FK-safe topological table order for seeding/truncating (cycles flagged), `format: "maintenance"` surfaces bloat/fragmentation/stale-statistics upkeep suggestions from engine catalogs (Postgres/MySQL), `format: "dictionary"` renders the whole schema as a Markdown data dictionary (column/type/PK/FK per table), and `format: "baseline_capture"` / `"baseline_compare"` record a row-count snapshot and diff later for per-table growth `format: "overview"` renders a one-call shape snapshot (tables/columns/indexes/FK edges/rows plus PII-name suspects), and `format: "pii_audit"` merges name-heuristic and content-scan PII findings into one deduplicated report |
| `health_<db_id>` | Report connectivity, ping latency, connection-pool state, engine stats (PostgreSQL buffer-cache hit ratio, MySQL InnoDB buffer efficiency), and recent PII-masking redaction counts when masking is active; `action: "trend"` renders the rolling pool-pressure history (last 20 samples with deltas) instead of a fresh check |
| `filter_tables` | Find tables whose names contain a substring; pass `value` instead to search every textual column of every table for a literal (e.g. locate which table holds an email or UUID) with per-column match counts |
| `transaction_<db_id>` | Multi-statement transactions (`begin`/`execute`/`commit`/`rollback`), pre-mutation safety net (every DELETE/UPDATE auto-captures affected rows; `list_snapshots`, `rollback_snapshot` undo a mutation), and schema lifecycle (`capture_schema_snapshot`, `check_schema_drift`, `list_schema_snapshots`) for migration verification |
| `execute_<db_id>` | Run write/DDL statements with `dry_run: true` for offline risk analysis (destructive ops, missing WHERE, rewrite/lock advisories); real executions of high/critical statements append an explicit risk notice; `script` runs a semicolon-separated multi-statement batch atomically (all commit or all roll back, failing statement named); `csv_data` + `csv_table` bulk-insert CSV content atomically (10k row cap); `migrate_dir` applies versioned `.sql` migration files in name order with per-migration atomicity and applied-tracking in `_mcp_migrations`; `copy_table` + `from_db` bulk-copies every row of a table from another database inside one transaction (`mask_pii: true` anonymizes PII-bearing text during the copy so prod data can seed staging safely); `verify_copy` + `from_db` reconciles row counts after a copy |

### TimescaleDB Tools

For PostgreSQL databases with TimescaleDB extension, these additional specialized tools are available:

| Tool Name | Description |
|-----------|-------------|
| `timescaledb_<db_id>` | Perform general TimescaleDB operations |
| `create_hypertable_<db_id>` | Convert a standard table to a TimescaleDB hypertable |
| `list_hypertables_<db_id>` | List all hypertables in the database |
| `time_series_query_<db_id>` | Execute optimized time-series queries with bucketing |
| `time_series_analyze_<db_id>` | Analyze time-series data patterns |
| `continuous_aggregate_<db_id>` | Create materialized views that automatically update |
| `refresh_continuous_aggregate_<db_id>` | Manually refresh continuous aggregates |

For detailed documentation on TimescaleDB tools, see [TIMESCALEDB_TOOLS.md](docs/TIMESCALEDB_TOOLS.md).

### Unified Tool Mode

If you connect many databases (5+), the per-database tool naming generates a large
number of tools (5 × N). Some MCP clients — Claude in particular — apply strict
limits on the total number of tools and tool description size that can cause the
agent to fail to load the server, ignore tools, or refuse to call them. Issue #18
documents this exact symptom: "the db-mcp-server does not function properly with
Claude, even though it works fine with OpenAI".

For these clients, launch the server with the `--unified-tools` flag to register six consolidated tools (`query`, `execute`, `transaction`, `performance`, `explain`, `describe`, `schema`, `filter_tables`) instead of per-database tools:

```bash
./bin/server -t stdio -c config.json --unified-tools
```

**Context-window cost (measured, `TestToolTokenBenchmark`):** unified mode costs ~1.1–1.3k tokens **regardless of how many databases are connected**, while per-database mode costs ~800 tokens per database (7 tools each) — 10 connected databases ≈ 8k tokens, a 6.3x difference. With one database only, per-database naming is slightly cheaper; unified wins from two databases onward and scales flat thereafter.
```

In unified mode, each tool accepts a required `database` parameter that names
which database the call should target. See the [Configuration](#configuration)
section for the full list of available databases. This dramatically reduces the
tool count and the cumulative description size, which resolves the Claude
compatibility issues.

For very large configurations, also enable `--lazy-loading` so that startup
doesn't open connections to databases that may never be queried during the
session.

## Examples

### Querying Multiple Databases

```sql
-- Query the MySQL database
query_mysql1("SELECT * FROM users LIMIT 10")

-- Query the PostgreSQL database in the same context
query_postgres1("SELECT * FROM products WHERE price > 100")

-- Query the SQLite database
query_sqlite_app("SELECT * FROM local_data WHERE created_at > datetime('now', '-1 day')")

-- Query the Oracle database
query_oracle_dev("SELECT * FROM employees WHERE hire_date > SYSDATE - 30")
```

### Managing Transactions

The `transaction_<db_id>` tool supports `begin`, `execute`, `commit`, and `rollback` actions. Each `begin` returns a `transactionId`; pass it back to stage statements and to commit or roll back:

```json
// 1. Start a transaction
{ "action": "begin" }
// → { "transactionId": "tx_mysql1_1730000000000000000" }

// 2. Execute statements within the transaction
{
  "action": "execute",
  "transactionId": "tx_mysql1_1730000000000000000",
  "statement": "INSERT INTO orders (customer_id, product_id) VALUES (1, 2)"
}

// 3a. Commit — persists all staged statements
{ "action": "commit", "transactionId": "tx_mysql1_1730000000000000000" }

// 3b. OR rollback — discards all staged statements
{ "action": "rollback", "transactionId": "tx_mysql1_1730000000000000000" }
```

Unknown or already-retired transaction IDs return a clear error instead of a silent success, so agents can detect and recover from lost-transaction situations.

### Exploring Database Schema

```sql
-- Get all tables in the database
schema_mysql1("tables")

-- Get columns for a specific table
schema_mysql1("columns", "users")

-- Get constraints
schema_mysql1("constraints", "orders")
```

### Working with SQLite-Specific Features

```sql
-- Create a table in SQLite
execute_sqlite_app("CREATE TABLE IF NOT EXISTS local_cache (key TEXT PRIMARY KEY, value TEXT, timestamp DATETIME)")

-- Use SQLite-specific date functions
query_sqlite_app("SELECT * FROM events WHERE date(created_at) = date('now')")

-- Query SQLite master table for schema information
query_sqlite_app("SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")

-- Performance optimization with WAL mode
execute_sqlite_app("PRAGMA journal_mode = WAL")
execute_sqlite_app("PRAGMA synchronous = NORMAL")
```

### Working with Oracle-Specific Features

```sql
-- Query user tables (excludes system schemas)
query_oracle_dev("SELECT table_name FROM user_tables ORDER BY table_name")

-- Use Oracle-specific date functions
query_oracle_dev("SELECT employee_id, hire_date FROM employees WHERE hire_date >= TRUNC(SYSDATE, 'YEAR')")

-- Oracle sequence operations
execute_oracle_dev("CREATE SEQUENCE emp_seq START WITH 1000 INCREMENT BY 1")
query_oracle_dev("SELECT emp_seq.NEXTVAL FROM DUAL")

-- Oracle-specific data types
query_oracle_dev("SELECT order_id, TO_CHAR(order_date, 'YYYY-MM-DD HH24:MI:SS') FROM orders")

-- Get schema metadata from Oracle data dictionary
query_oracle_dev("SELECT column_name, data_type, nullable FROM user_tab_columns WHERE table_name = 'EMPLOYEES'")

-- Use Oracle analytic functions
query_oracle_dev("SELECT employee_id, salary, RANK() OVER (ORDER BY salary DESC) as salary_rank FROM employees")
```

## Troubleshooting

### Common Issues

- **Connection Failures**: Verify network connectivity and database credentials
- **Permission Errors**: Ensure the database user has appropriate permissions
- **Timeout Issues**: Check the `query_timeout` setting in your configuration

### Logs

Enable verbose logging for troubleshooting:

```bash
./bin/server -t sse -c config.json -v
```

## Testing

### Running Tests

The project includes comprehensive unit and integration tests for all supported databases.

#### Unit Tests

Run unit tests (no database required):

```bash
make test
# or
go test -short ./...
```

#### Integration Tests

Integration tests require running database instances. We provide Docker Compose configurations for easy setup.

**Test All Databases:**

```bash
# Start test databases
docker-compose -f docker-compose.test.yml up -d

# Run all integration tests
go test ./... -v

# Stop test databases
docker-compose -f docker-compose.test.yml down -v
```

**Test Oracle Database:**

```bash
# Start Oracle test environment
./oracle-test.sh start

# Run Oracle tests
./oracle-test.sh test
# or manually
ORACLE_TEST_HOST=localhost go test -v ./pkg/db -run TestOracle
ORACLE_TEST_HOST=localhost go test -v ./pkg/dbtools -run TestOracle

# Stop Oracle test environment
./oracle-test.sh stop

# Full cleanup (removes volumes)
./oracle-test.sh cleanup
```

**Test TimescaleDB:**

```bash
# Start TimescaleDB test environment
./timescaledb-test.sh start

# Run TimescaleDB tests
TIMESCALEDB_TEST_HOST=localhost go test -v ./pkg/db/timescale ./internal/delivery/mcp

# Stop TimescaleDB test environment
./timescaledb-test.sh stop
```

#### Regression Tests

Run comprehensive regression tests across all database types:

```bash
# Ensure all test databases are running
docker-compose -f docker-compose.test.yml up -d
./oracle-test.sh start

# Run regression tests
MYSQL_TEST_HOST=localhost \
POSTGRES_TEST_HOST=localhost \
ORACLE_TEST_HOST=localhost \
go test -v ./pkg/db -run TestRegression

# Run connection pooling tests
go test -v ./pkg/db -run TestConnectionPooling
```

### Continuous Integration

All tests run automatically on every pull request via GitHub Actions. The CI pipeline includes:

- **Unit Tests**: Fast tests that don't require database connections
- **Integration Tests**: Tests against MySQL, PostgreSQL, SQLite, and Oracle databases
- **Regression Tests**: Comprehensive tests ensuring backward compatibility
- **Linting**: Code quality checks with golangci-lint

## Contributing

We welcome contributions to the DB MCP Server project! To contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please see our [CONTRIBUTING.md](docs/CONTRIBUTING.md) file for detailed guidelines.

### Testing Your Changes

Before submitting a pull request, please ensure:

1. All unit tests pass: `go test -short ./...`
2. Integration tests pass for affected databases
3. Code follows the project's style guidelines: `golangci-lint run ./...`
4. New features include appropriate test coverage

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
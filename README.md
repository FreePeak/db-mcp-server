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

<div align="center">
  <a href="docs/PRD.md"><b>📄 Product Requirements — current capabilities, roadmap & decisions</b></a>
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
| `"max_rows": 1000` | per database | Truncates result sets at N rows and appends an explicit `[Truncated]` notice so the model knows to refine its query instead of losing context. `0` (default) means unlimited. |
| `"masking_rules": [...]` | per database | Masks values of result columns whose **name** matches a rule's regex before they leave the server — applies to every query shape including `SELECT *`. Strategies: `"fixed_string"` (replace with `value`), `"null"`, and `"partial"` (`keep_last` trailing characters visible; shorter values fully masked). First matching rule wins; invalid patterns or unknown strategies abort config load (fail closed); masked-cell counts are reported in the result footer. Renaming a column with an alias bypasses name matching by design. See [docs/design/column-masking-scoping.md](docs/design/column-masking-scoping.md). |
| `"query_timeout": 30` | per database | Cancels statements that exceed the timeout in seconds; enforced at the repository layer for every tool (queries, statements, transactions, explain, schema inspection). Unset defaults to 30s; `-1` disables. Env-only deployments can set `QUERY_TIMEOUT_SECONDS` to fill connections without an explicit value (JSON keeps precedence). |

| `DB_MCP_AUDIT_LOG=/path/audit.jsonl` | process | Appends one JSONL record per executed statement — timestamp, op (`query`/`execute`/`tx_*`), database, statement (capped at 10k chars), duration, error. Includes rejected attempts against read-only databases. Best-effort writes never fail a query; file is created with `0600`. |

> **Defense in depth**: read-only is enforced in three layers — application classifier, engine session defaults, and (recommended) least-privilege database users. Oracle currently relies on the classifier plus user privileges.

## Supported Databases

| Database   | Status                    | Features                                                     |
| ---------- | ------------------------- | ------------------------------------------------------------ |
| MySQL      | ✅ Full Support           | Queries, Transactions, Schema Analysis, Performance Insights |
| PostgreSQL | ✅ Full Support (v9.6-17) | Queries, Transactions, Schema Analysis, Performance Insights |
| SQLite     | ✅ Full Support           | File-based & In-memory databases, SQLCipher encryption support |
| Oracle     | ✅ Full Support (10g-23c) | Queries, Transactions, Schema Analysis, RAC, Cloud Wallet, TNS |
| TimescaleDB| ✅ Full Support           | Time-Series Queries, Hypertable Discovery (write policies via SQL) |

## Deployment and transport status

The safest local integration is **STDIO**. HTTP transports are still under hardening; do not expose the current server directly to a shared network.

### Personal/local STDIO (recommended)

```bash
# Build from the audited source revision
make build

# Use a private, single-connection config
./bin/server -t stdio -c /absolute/path/to/config.local.json
```

For Cursor, point the client at the actual build output and use absolute paths:

```json
{
  "mcpServers": {
    "local-db": {
      "command": "/absolute/path/to/db-mcp-server/bin/server",
      "args": ["-t", "stdio", "-c", "/absolute/path/to/config.local.json"]
    }
  }
}
```

Use a read-only database account and a bounded `query_timeout` for a personal
PostgreSQL/MySQL connection. For SQLite, use an absolute `database_path` and
`"read_only": true` when the agent only needs to inspect data.

### Legacy SSE (compatibility only)

The historical Cortex SSE transport has no built-in authentication. It is
**disabled by default** and must only be enabled explicitly behind a trusted
private reverse proxy:

```bash
./bin/server -t sse -h 127.0.0.1 -p 9092 \
  -allow-insecure-sse -c /absolute/path/to/config.local.json
```

Do not use this mode for a shared deployment. The legacy endpoint does not
apply `DB_MCP_API_KEY`; terminate TLS, authentication, and network policy at the
proxy. A future release will replace it with a maintained Streamable HTTP
implementation.

### Streamable HTTP (experimental)

The `streamable` endpoint is backed by the maintained Go MCP SDK and supports
stateful sessions, JSON or SSE responses, notifications, GET/DELETE session
lifecycle, and JSON-RPC dispatch. The process refuses to start it without
`DB_MCP_API_KEY`; keep it behind a private TLS gateway until the identity and
conformance rollout is complete.

For browser-origin requests, set an explicit comma-separated allowlist:

```bash
MCP_ALLOWED_ORIGINS=https://agent.example.com
```

The built-in static bearer key is a bounded pilot mechanism. It grants
`db:read` by default; set `DB_MCP_API_SCOPES=db:read,db:write` only for a
trusted write client. `db:admin` is reserved for controlled administration.
Set `DB_MCP_OIDC_ISSUER` and `DB_MCP_OIDC_AUDIENCE` to use OIDC discovery,
JWKS signature validation, issuer/audience checks, and subject identity for
shared deployments; database-specific scopes are still intentionally not
enabled until that policy is defined.

### Docker development image

The published image is a development/demo artifact, not an enterprise
hardening recommendation. The current Dockerfile runs as an unprivileged
user, omits an embedded config, and includes the legacy
`-allow-insecure-sse` behavior for local Compose compatibility. It exposes a
separate health port (`9093`) and is not suitable for public publication.

```bash
# Build a local image from this source revision
docker build -t db-mcp-server:local .

# Keep it private; the image's default SSE mode is compatibility-only
docker run --rm -p 127.0.0.1:9092:9092 \
  -p 127.0.0.1:9093:9093 \
  -v "$(pwd)/config.local.json:/app/config.json:ro" \
  db-mcp-server:local
```

For a team deployment, wait for the authenticated Streamable transport and use
a non-root, read-only-filesystem image with a private network, external secret
store, and a TLS/OIDC gateway.

### Source Code Installation

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Build the server
make build

# Run the server
./bin/server -t stdio -c config.json
```

## Configuration

### Database Configuration File

Create a `config.json` file with your database connections. `schema_version: 1`
enables strict unknown-field validation:

```json
{
  "schema_version": 1,
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "mysql1",
      "port": 3306,
      "name": "db1",
      "user": "user1",
      "password_env": "DB_MCP_MYSQL_PASSWORD",
      "query_timeout": 60,
      "max_open_conns": 20,
      "max_idle_conns": 5,
      "conn_max_lifetime_seconds": 300,
      "conn_max_idle_time_seconds": 60,
      "read_only": false,
      "max_rows": 1000,
      "masking_rules": [
        { "pattern": "(?i)email", "strategy": "fixed_string", "value": "***MASKED***" },
        { "pattern": "(?i)(ssn|tax_id)", "strategy": "null" }
      ]
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "postgres1",
      "port": 5432,
      "name": "db1",
      "user": "user1",
      "password_env": "DB_MCP_POSTGRES_PASSWORD"
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

#### Secret references

Do not put production credentials directly in JSON. Use either an
environment variable name or a mounted secret file per connection:

```json
{
  "id": "prod",
  "type": "postgres",
  "host": "db.internal",
  "port": 5432,
  "name": "app",
  "user": "mcp_reader",
  "password_env": "DB_MCP_PROD_PASSWORD"
}
```

`password_file` and `encryption_key_env`/`encryption_key_file` are also
supported. Relative file paths resolve from the config file directory; an
inline value cannot be combined with a reference. The resolved value stays in
process memory and is not printed by the config loader.

### Command-Line Options

```bash
# Basic syntax
./bin/server -t <transport> -c <config-file>

# HTTP transport options (legacy SSE requires an explicit opt-in)
./bin/server -t sse -h <hostname> -p <port> -allow-insecure-sse -c <config-file>

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
- `-t, -transport`: Transport mode (`stdio`, legacy `sse`, or experimental `streamable`)
- `-c, -config`: Path to database configuration file
- `-p, -port`: HTTP port (default: 9092)
- `-h, -host`: HTTP bind host (default: `127.0.0.1`)
- `-allow-insecure-sse`: Explicitly allow legacy SSE for a private development proxy
- `-api-key`: Bearer key for the experimental Streamable HTTP entrypoint (or use `DB_MCP_API_KEY`)
- `-health-port`: HTTP health port (default: 9093; `0` disables it)
- `-log-level`: Log level (`debug`, `info`, `warn`, `error`)
- `-log-dir`: Directory for log files (default: `./logs` in current directory)
- `-db-config`: Inline JSON database configuration
- `-lazy-loading`: Establish connections on first use
- `-unified-tools`: Register one tool set with a `database` parameter

### Environment Variables

Values in a `.env` file are loaded first; real environment variables take precedence. A JSON config file (`CONFIG_PATH`/`DB_CONFIG_FILE`) takes precedence over inline and legacy database variables.

| Variable | Default | Purpose |
|---|---|---|
| `CONFIG_PATH` / `DB_CONFIG_FILE` | `config.json` | Path to the multi-database JSON config |
| `DB_CONFIG` | — | Inline JSON database configuration (used when no file is present) |
| `TRANSPORT_MODE` | `stdio` | Transport mode when `-t` is not passed |
| `SERVER_PORT` | `9092` | HTTP port |
| `MCP_ALLOWED_ORIGINS` | empty | Comma-separated browser origins allowed by experimental Streamable HTTP |
| `DB_MCP_API_KEY` | empty | Bearer key for experimental Streamable HTTP; keep it in a secret manager |
| `DB_MCP_API_SCOPES` | `db:read` | Required/granted scopes: `db:read`, `db:write`, and/or `db:admin` |
| `DB_MCP_OIDC_ISSUER` | empty | OIDC issuer URL; when set, OIDC/JWT verification replaces the static pilot key |
| `DB_MCP_OIDC_AUDIENCE` | empty | Required OIDC audience/client ID |
| `LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `DISABLE_LOGGING` | `false` | `true`/`1` silences logging entirely |
| `DB_TYPE`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | engine defaults | Single-database fallback when no JSON config exists |
| `QUERY_TIMEOUT_SECONDS` | unset | Fills connections that don't set their own `query_timeout`; negative disables the cap. JSON configs keep precedence. |
| `DB_MCP_AUDIT_LOG` | unset | Optional append-only JSONL audit path |
| `VERBOSE` | unset | `true`/`1` sets the startup log level to `debug` (the default is `error`, i.e. silent unless `LOG_LEVEL` is set) |
| `MCP_DISABLE_LOGGING` | unset | `true`/`1` discards MCP protocol logs (writer-level switch, independent of `LOG_LEVEL`/`DISABLE_LOGGING`) |


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
| `query_<db_id>` | Execute SELECT queries and get results as a tabular dataset |
| `execute_<db_id>` | Run data manipulation statements (INSERT, UPDATE, DELETE) |
| `transaction_<db_id>` | Begin, commit, and rollback transactions |

### Schema Tools

| Tool Name | Description |
|-----------|-------------|
| `schema_<db_id>` | Get information about tables, columns, indexes, and foreign keys |
| `generate_schema_<db_id>` | Generate SQL or code from database schema |

### Performance Tools

| Tool Name | Description |
|-----------|-------------|
| `performance_<db_id>` | Analyze query performance via actions: `stats` / `slow_queries` (in-process tracker), `engine_slow_queries` (pg_stat_statements / MySQL digest tables / Oracle v$sqlarea), `suggest` (static SQL lint), `suggest_indexes` (heuristic CREATE INDEX advice for one statement, equality-first composites, verify with EXPLAIN), `validate_suggestions` (PostgreSQL: installs the same suggestions as cost-free hypothetical indexes via the hypopg extension and reports whether the planner actually picks each one — ground-truth validation instead of manual EXPLAIN), `workload_suggestions` (same analysis across the top-N expensive workload statements, weighted by executions), `index_health` (duplicate/redundant/unused/invalid indexes and table bloat findings from catalogs; usage evidence where engine statistics exist), `db_health` (everything index_health covers plus connection-pressure utilization vs max_connections), `reset` |
| `explain_<db_id>` | Show the execution plan for a SQL statement without running it; `analyze: true` executes with timing/buffer stats (PostgreSQL/MySQL). Writes stay blocked on read-only databases |
| `describe_<db_id>` | Inspect one table's columns, indexes, and row estimate via engine catalog queries |
| `health_<db_id>` | Report connectivity, ping latency, connection-pool state, and engine stats (PostgreSQL buffer-cache hit ratio, MySQL InnoDB buffer efficiency) |

### TimescaleDB Tools

For PostgreSQL databases with the `timescaledb` extension installed, these additional
specialized tools are registered automatically at startup (registration is config-driven,
so it also works under `--lazy-loading`; each handler verifies the extension at call time
and returns an actionable error when it is absent):

| Tool Name | Description |
|-----------|-------------|
| `timescaledb_timeseries_query_<db_id>` | Execute optimized time-series queries with time bucketing (`time_bucket`), filtering, and window functions |
| `timescaledb_analyze_timeseries_<db_id>` | Analyze time-series patterns (trend, seasonality summary) for one table/column |
| `timescaledb_list_hypertables_<db_id>` | List hypertables with their time column and dimension count (read-only) |
| `timescaledb_compression_settings_<db_id>` | Show compression configuration for hypertables (read-only) |
| `timescaledb_retention_policy_<db_id>` | Show configured retention policies (read-only) |
| `timescaledb_list_continuous_aggregates_<db_id>` | List continuous aggregates with bucket interval and refresh policy (read-only) |
| `timescaledb_continuous_aggregate_info_<db_id>` | Inspect one continuous aggregate in detail (read-only) |

In unified mode the same seven tools appear once as `timescaledb_timeseries_query`,
`timescaledb_analyze_timeseries`, `timescaledb_list_hypertables`,
`timescaledb_compression_settings`, `timescaledb_retention_policy`,
`timescaledb_list_continuous_aggregates`, and `timescaledb_continuous_aggregate_info`,
each taking a required `database` parameter.

> **Scope note**: read-only discovery above goes through the query pipeline and therefore
> stays usable on `read_only` databases; each handler checks for the `timescaledb`
> extension first. Write-policy operations (hypertable creation, compression toggles,
> add/remove retention or refresh policies) remain unexposed — use plain SQL through
> the query/execute tools in the meantime. For detailed documentation, see
> [TIMESCALEDB_TOOLS.md](docs/TIMESCALEDB_TOOLS.md).

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

**Context-window cost (measured, `TestToolTokenBenchmark`, re-verified 2026-08 via `scripts/token-benchmark.sh`):** unified mode costs ~1.25–1.6k tokens **regardless of how many databases are connected**, while per-database mode costs ~800 tokens per database (7 tools each) — 10 connected databases ≈ 8k tokens, an 80% wire-payload saving with unified. With one database only, per-database naming is slightly cheaper; unified wins from two databases onward and scales flat thereafter. Re-measure the real wire payload yourself with `scripts/token-benchmark.sh`; methodology and results in [docs/benchmark-token-efficiency.md](docs/benchmark-token-efficiency.md).
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

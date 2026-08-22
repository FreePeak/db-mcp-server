# Changelog

## [v1.9.0] - 2026-04-11

### Added
- Oracle Database support (10g–23c) with RAC, Cloud Wallet, and TNS authentication (#51)
- `filter_table_names` tool for listing/filtering table names across databases
- npm distribution channel with release automation (#48)

### Fixed
- SQLite no longer creates a stray file named after the connection ID in the working directory (#56)
- `-log-dir` flag now respected in stdio mode (#45)
- Response format for empty result sets (#30)
- npm publish workflow no longer runs a redundant install step (#50)

## [v1.8.0] - 2025-05-09

### Added
- TimescaleDB tool category: hypertable creation and listing (TOOL-1/2/3)
- TimescaleDB compression policy tools (TOOL-4)
- TimescaleDB retention policy tools (TOOL-5)
- TimescaleDB time-series query tools and continuous aggregates (TOOL-6/7)
- Editor context integration: automatic TimescaleDB detection, hypertable schema info, function/query suggestions (CTX-1/2/4/5)
- Docker test environment for TimescaleDB (TEST-1)

### Fixed
- Glama/Docker image build issues

## [v1.7.0] - 2025-04-24

### Added
- Per-database query timeout configuration (`query_timeout`) (#31)
- Multi-arch Docker images (amd64/x86) (#27)

## [Unreleased]

### Added
- `max_rows` per-database configuration option (all engines): truncates query results with an explicit `[Truncated]` notice to protect agent context windows from large result sets
- **Engine-level read-only enforcement** (PostgreSQL/TimescaleDB and MySQL): when `read_only: true`, connections are opened with server-side write rejection (`default_transaction_read_only=on` / `transaction_read_only=1`), so guardrails hold even if application-layer checks are bypassed. SQLite already enforces via `mode=ro`; Oracle relies on the application-layer classifier plus least-privilege users.
- New `explain_<db_id>` tool (unified mode: `explain`): shows the engine's execution plan for a statement without executing it — `EXPLAIN` (PostgreSQL/TimescaleDB), `EXPLAIN ANALYZE` opt-in (MySQL), `EXPLAIN QUERY PLAN` (SQLite), two-step `EXPLAIN PLAN FOR` + `DBMS_XPLAN.DISPLAY` (Oracle)
- New `describe_<db_id>` tool (unified mode: `describe`): per-table metadata inspection — columns, indexes, and row estimates via engine-appropriate catalog queries, with identifier validation against catalog-query injection
- `describe_<db_id>` also surfaces constraints (PRIMARY KEY / FOREIGN KEY / UNIQUE) from engine constraint catalogs, best-effort so introspection gaps never fail the describe
- New `health_<db_id>` tool (unified mode: `health`): connectivity probe with ping latency, Go `database/sql` pool pressure (open/in-use/idle/wait count and duration), and best-effort engine indicators — PostgreSQL buffer-cache hit ratio, MySQL InnoDB buffer efficiency

### Changed
- `performance_<db_id>` tool now returns real data instead of placeholder text: tracked query metrics (count/avg/max/min per normalized statement), recorded slow queries with errors, static SQL issue suggestions (select-star, cartesian joins, missing WHERE, etc.), and history reset — wired to the query-tracking analyzer that already instruments every `query_*` execution
- `performance_<db_id>` gains `engine_slow_queries` action: top statements by execution time from the database's own catalogs — `pg_stat_statements` (PostgreSQL/TimescaleDB) and `performance_schema` digests (MySQL) — with actionable degradation notes when extensions or grants are missing

### Fixed
- **Read-only bypass (security)**: write statements (`INSERT`/`UPDATE`/`DELETE`/DDL/data-modifying CTEs/stacked statements) executed through the `query_*` tool no longer bypass the per-database `read_only: true` guard; statement classification strips comments and string literals and defaults to deny for unrecognized leading keywords
- **Transactions were stubbed**: the `transaction_*` tools' `begin` action silently committed immediately, while `execute`, `commit`, and `rollback` returned success without doing anything. All four actions now operate on a real stored transaction keyed by the returned `transactionId`; unknown IDs fail with a clear error instead of faking success

## [v1.6.1] - 2025-04-01

### Added
- OpenAI Agents SDK compatibility by adding Items property to array parameters
- Test script for verifying OpenAI Agents SDK compatibility

### Fixed
- Issue #8: Array parameters in tool definitions now include required `items` property
- JSON Schema validation errors in OpenAI Agents SDK integration

## [v1.6.0] - 2023-03-31

### Changed
- Upgraded cortex dependency from v1.0.3 to v1.0.4

## [] - 2023-03-31

### Added
- Internal logging system for improved debugging and monitoring
- Logger implementation for all packages

### Fixed
- Connection issues with PostgreSQL databases
- Restored functionality for all MCP tools
- Eliminated non-JSON RPC logging in stdio mode

## [] - 2023-03-25

### Added
- Initial release of DB MCP Server
- Multi-database connection support
- Tool generation for database operations
- README with guidelines on using tools in Cursor


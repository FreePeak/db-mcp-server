# Changelog
## [Unreleased]
### Added
- TimescaleDB discovery tool: `timescaledb_list_hypertables_<db_id>` (and unified `timescaledb_list_hypertables`) listing hypertables with time column and dimensions; auto-detected via the `timescaledb` extension at startup
### Fixed
- Read-only safety for the wired TimescaleDB tools: `list_hypertables`, `time_series_query`, and `analyze_time_series` executed their SELECTs through the write-classified statement path and failed on `"read_only": true` databases; they now use the query path
- README TimescaleDB section advertised seven tools when only two were registered; it now documents the three wired tools and states that write-policy operations remain SQL-only
## [v1.12.0] - 2026-08-25
### Added
- Engine-level read-only enforcement for Oracle via fail-closed privilege auditing (`read_only` databases refuse credentials holding write privileges; completes the four-engine guarantee started in v1.10.0)
- Performance analysis actions: engine-level slow queries (`pg_stat_statements` / MySQL performance_schema digests / Oracle `v$sqlarea`), `index_health` (duplicate/redundant/unused/invalid indexes, table bloat/fragmentation), and unified `db_health` with connection-pressure findings
- Index advisor: `suggest_indexes` with composite synthesis (equality-first, sort appended), join/alias resolution, constraint-aware coverage (PK/UNIQUE columns count as covered), and workload-driven suggestions weighted by execution count then total engine time (duration-ranked when engines report time)
- Planner-validated index suggestions on PostgreSQL via hypopg (`validate_suggestions`): hypothetical indexes + EXPLAIN verdicts replace manual verification
- Index suggestions appended to `explain` output and the slow-queries view
- Statement timeout enforcement: per-database `query_timeout`, env-only deployments covered by `QUERY_TIMEOUT_SECONDS`; timeout/guardrail visibility in db_health
- JSONL audit trail: `DB_MCP_AUDIT_LOG` records every executed statement (op, database, statement capped at 10k chars, duration, error) including rejected read-only attempts
- Name-based column masking: `fixed_string`/`null`/`partial` strategies, fail-closed config validation, masked-cell counts in results
- Foreign-key referenced-table resolution in describe output; database-wide Mermaid ERD via performance tool `format=mermaid`
- README environment-variables reference; token-efficiency benchmark re-measured via wire-payload harness (`scripts/token-benchmark.sh`)
- Live-engine regression tests in CI; repeatable harnesses: `scripts/live-db-setup.sh`, `scripts/smoke.sh`

### Fixed
- MySQL `[]byte` driver cells are decoded before partial masking (found by live validation)
- Constraint-backed primary keys excluded from UNUSED-index advice
- `sys.schema_unused_indexes` column mapping corrected against live MySQL

## [v1.11.0] - 2026-08-22
Tagged alongside v1.10.0 for distribution-pipeline verification; no functional changes beyond v1.10.0.

## [v1.10.0] - 2026-08-22
### Added
- Production guardrails pack (#85): query-tool read-only bypass closed, `max_rows` result cap, real stored transactions replacing stubs
- Engine-level read-only enforcement for PostgreSQL (`default_transaction_read_only=on`) and MySQL (`transaction_read_only=1`)
- `explain_<db_id>` tool for execution-plan analysis
- `describe_<db_id>` tool for per-table schema inspection with PK/FK constraint surfacing
- Real performance tool backed by a query-metrics analyzer (placeholder removed)
- Streamable HTTP transport (#34)
- Bearer-token API key middleware for HTTP transports (#57)

### Fixed
- Prompts list behavior (#35)

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
- `describe_<db_id>` also surfaces constraints (PRIMARY KEY / FOREIGN KEY / UNIQUE) from engine constraint catalogs, best-effort so introspection gaps never fail the describe; foreign keys resolve to their referenced table and column (`author_id -> authors(id)`)
- New `health_<db_id>` tool (unified mode: `health`): connectivity probe with ping latency, Go `database/sql` pool pressure (open/in-use/idle/wait count and duration), and best-effort engine indicators — PostgreSQL buffer-cache hit ratio, MySQL InnoDB buffer efficiency

### Changed
- `performance_<db_id>` tool now returns real data instead of placeholder text: tracked query metrics (count/avg/max/min per normalized statement), recorded slow queries with errors, static SQL issue suggestions (select-star, cartesian joins, missing WHERE, etc.), and history reset — wired to the query-tracking analyzer that already instruments every `query_*` execution
- `performance_<db_id>` gains `engine_slow_queries` action: top statements by execution time from the database's own catalogs — `pg_stat_statements` (PostgreSQL/TimescaleDB) and `performance_schema` digests (MySQL) — with actionable degradation notes when extensions or grants are missing
- `schema_<db_id>` accepts `format=mermaid`: renders the database's foreign-key relationships as a Mermaid `erDiagram`, giving agents an at-a-glance entity-relationship map without describing every table

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


# Changelog
## [Unreleased]

### Added
- **OAuth protected resource metadata** (`DB_MCP_RESOURCE_URL` with `DB_MCP_OIDC_ISSUER`): Streamable HTTP serves RFC 9728 metadata at `/.well-known/oauth-protected-resource` so MCP clients can discover the identity provider, and `/mcp` answers 401/403 with a `WWW-Authenticate` challenge pointing at it
- **Server-side row cap (Auto-LIMIT)**: when a connection sets `max_rows`, a single SELECT or WITH statement sent through the query tool gets `LIMIT max_rows+1` appended on PostgreSQL (including TimescaleDB), MySQL and SQLite, so the database stops producing rows past the cap instead of streaming the whole result to be truncated client-side; the existing `Truncated: showing first N rows` notice still fires. Statements that already bound or lock their rows (`LIMIT`, `OFFSET`, `FETCH`, `FOR`, `INTO`, `LOCK`), compound statements ending in a `VALUES` list, multi-statement input and non-SELECT statements are left unchanged, as are statements containing a backslash, `#` or a MySQL `/*!` comment, which the literal stripper cannot model. Select-list functions with side effects (for example `nextval()`) run for at most max_rows+1 rows. Internal tool queries are never rewritten, Oracle is not rewritten (client-side truncation still applies there), and when a capped query fails the error names the appended LIMIT so an explicit LIMIT can be added
- **Estimated-cost gate** (`max_query_cost` per connection, PostgreSQL and MySQL): a SELECT or WITH sent through the query tool is denied when its EXPLAIN estimate exceeds the limit; an unreadable estimate denies the query (fail closed); `explain` with `analyze=true` is checked first, and a raw `EXPLAIN ANALYZE` through the query tool is refused. Units are planner cost units, not time, so calibrate the limit with the explain tool; 0 (the default) turns the gate off, and an unsupported engine fails at startup
- **Audit record attribution**: `DB_MCP_AUDIT_LOG` records gain optional `principal` (authenticated caller), `session` (MCP session ID) and `rows` (rows shown for a query, rows affected for a write) fields; each is omitted when unknown, so existing consumers keep working
- **Per-agent API keys**: `DB_MCP_AGENT_KEYS_FILE` gives each agent its own key (stored as a SHA-256 digest), identity, scopes and database allowlist on Streamable HTTP. Scope and allowlist are checked on every tool call, and the audit record names the key `id` as the principal. Agent keys require the `-unified-tools` flag, because they can call only unified tool names; the server refuses to start with the file set and the flag off. TimescaleDB tools are not available to agent keys, and `list_databases` lists only the allowlisted databases
- **Audit tool name**: `DB_MCP_AUDIT_LOG` records gain a `tool` field with the MCP tool name of the call
- **Audit denied calls**: a tool call refused by the tool scope check or an agent key database allowlist is recorded with op `denied`, the reason in `error`, and the same target and caller fields as an executed call
- **Audit default principal**: `DB_MCP_AUDIT_PRINCIPAL` labels audit records that have no authenticated principal, such as stdio sessions; a token principal always wins
- **Mandatory audit log**: `DB_MCP_AUDIT_REQUIRED=true` (or `1`) makes startup fail unless `DB_MCP_AUDIT_LOG` is set and its file can be opened
- **OIDC role grants** (`DB_MCP_OIDC_ROLES_FILE`): a JSON file maps one token claim to roles, each a grant of scopes, a database allowlist and an optional `max_query_cost`; the first matching role in file order applies, and a token matching no role is denied everything. Agent keys accept the same `max_query_cost`, and the lower of a connection limit and a caller limit is enforced. A roles file requires `-unified-tools`, and the server refuses to start without it

### Changed (BREAKING)
- **The Docker image defaults to authenticated Streamable HTTP**: `TRANSPORT_MODE` now defaults to `streamable`, and the container refuses to start without an API key or OIDC configuration. The API key may come from `DB_MCP_API_KEY` or `DB_MCP_API_KEY_FILE`. To migrate, either provide an API key (`DB_MCP_API_KEY` or `DB_MCP_API_KEY_FILE`) or OIDC configuration (`DB_MCP_OIDC_ISSUER` and `DB_MCP_OIDC_AUDIENCE`); or, for a private development setup only, opt into legacy SSE, which has no built-in authentication, by setting both `TRANSPORT_MODE=sse` and `ALLOW_INSECURE_SSE=true`

### Fixed
- **Read-only PostgreSQL connections keep the read-only session default when `options` is set**: a connection with `read_only: true` and its own `options` value now gets `-c default_transaction_read_only=on` appended to that value instead of losing it, so both the operator's flags and the read-only default apply

## [v1.14.1] - 2026-11-17
### Fixed
- Special characters in credentials no longer corrupt connection strings (#106): Oracle DSNs are assembled through URL userinfo encoding, so a password like `pa#ss` (or any `# @ / % ? :` character) connects instead of failing with an invalid-port or escape parse error; PostgreSQL key=value values (host, user, password, dbname, SSL cert/key/CA paths) containing spaces, single quotes or backslashes are quoted per lib/pq's parser rules — passwords like `pa ss`, `'pass` and `pa\ss` previously truncated or corrupted the DSN. MySQL DSN building was verified already correct and now has regression coverage

## [v1.14.0] - 2026-10-13
### Added
- **Content-based PII masking** (`"mask_pii": true` per connection, optional `mask_pii` per query): redacts emails, phone numbers, Luhn-checked card numbers, SSNs, IPv4 addresses and long numeric identifiers by *content shape* — including inside free-text columns no rule covers — plus whole-cell redaction for columns named like PII carriers. Operator-controlled: once enabled, the per-query parameter can only add masking, never remove it
- **`dry_run` pre-flight on the execute tool**: classifies a statement offline (kind, risk level, advisories) and executes nothing; works even when the database is unreachable
- **Post-execution risk advisories**: a statement that runs and classifies at or above the warning threshold (default high) carries a `⚠ Risk notice` in its result — DROP targets, TRUNCATE, UPDATE/DELETE with no top-level WHERE, column drops and type-change rewrites (with the engine's live row estimate)
- **Pre-mutation snapshots + `rollback_snapshot`**: every `DELETE`/`UPDATE` through the execute tool captures the rows it will affect (bounded ring of 25 per database) and reports the snapshot ID; the transaction tool gains `list_snapshots` and `rollback_snapshot` for agent-driven undo
- `db-mcp-server update`: replaces the running binary with the latest published release — resolves the tag, verifies the download against the release's `SHA256SUMS` manifest, and swaps the file atomically; `-check` reports without downloading, `-force` reinstalls, `-version` pins a release
- `--version` prints the build version, and MCP clients now see the real release version in `serverInfo` instead of a hardcoded `1.0.0`
### Fixed
- The release `-ldflags="-X main.version=…"` the Makefile and `npm-publish.yml` pass was a silent no-op (`cmd/server` declared no `version` variable), so every release binary reported `1.0.0`
- Tool-type resolution takes the exact name before the underscore-prefix split, so a multi-word tool type is never shadowed by a shorter registered prefix
- `TestWorkloadIndexSuggestions_EndToEnd` depended on test ordering: the process-global performance analyzer was not reset, so a stale tracker window could drop the composite suggestion (~1 run in 8)

## [v1.13.0] - 2026-10-12
### Added
- TimescaleDB discovery tools: `timescaledb_list_hypertables_<db_id>` (and unified `timescaledb_list_hypertables`) listing hypertables with time column and dimensions, plus compression settings, retention policy and continuous-aggregate discovery — auto-detected via the `timescaledb` extension at startup
- Lazy-loading-safe TimescaleDB registration: with `--lazy-loading` the tools register on PostgreSQL config type alone (no startup connection); handlers verify the `timescaledb` extension at call time and degrade with actionable guidance when it is absent or unverifiable
- Hardening for personal + enterprise use (#88): single canonical config object (fixes `max_rows`/masking/`query_timeout` silently dropped by the second config parser), strict JSON config validation, real transaction lifetimes held server-side, SDK-backed streamable HTTP with Origin allowlisting, bearer API-key auth, and OIDC verification; the container now runs unprivileged
- CI: `Verify source` job (test + vet) gates the publish workflows; release images carry provenance and SBOM attestations; publish guards are scoped to tag/dispatch events so a plain push to main never fails on missing registry secrets
### Fixed
- Tool-type resolution takes the exact name before the underscore-prefix split (#92, from #89): `list_databases` no longer resolves to the directory `list` tool, with regression coverage for the collision
- Read-only safety for the wired TimescaleDB tools: `list_hypertables`, `time_series_query`, and `analyze_time_series` executed their SELECTs through the write-classified statement path and failed on `"read_only": true` databases; they now use the query path
- TimescaleDB information-view queries rewritten against modern catalogs (live-engine validated); README TimescaleDB section now documents the three wired tools and states that write-policy operations remain SQL-only
- The arbitrary server-directory `list` tool is no longer registered on MCP clients (#88)
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


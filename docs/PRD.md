# db-mcp-server — Product Requirements Document

The single source of truth for this server: what it is, what it does, how it
is built, and where it is going. Every other document (README quick-start,
`docs/cycles/` narratives, engine deep-dives) is linked from here, never
duplicated.

Status: **maintained, active** (last substantive feature release v1.13.0,
2026-10-12; period-review 2026-10) · License MIT · Repo
`github.com/FreePeak/db-mcp-server`

*Last updated: 2026-10-12 (Docker Hub publishing live; SQLite PRAGMA gap check closed)*

## 1. What this is

A multi-database **Model Context Protocol (MCP) server** that gives AI
assistants structured, guarded access to databases through a fixed set of
MCP tools. One process connects to many databases at once — currently
**PostgreSQL (+ TimescaleDB), MySQL, SQLite, and Oracle** — and exposes a
uniform tool surface over all of them.

Positioning: a **database-ops tool for agents**, not a general query proxy.
The differentiators vs the competitive baseline (DBHub/Bytebase, MCP
Toolbox, Postgres MCP Pro) are the governance pack: engine-level read-only
enforcement, result caps, timeouts, PII masking, and an audit trail.

### 1.1 Goals (why it exists)

- G1 — Let agents **discover and query real databases safely**: schema
  introspection, query, explain, performance analysis.
- G2 — **Protect the database from the agent**: read-only-by-default,
  statement classification, max-row caps, per-query timeouts, masking.
- G3 — **Protect the operator from the agent**: JSONL audit trail, guardrail
  visibility, least-privilege guidance.
- G4 — **Scale to many databases** without blowing the model context window
  (unified tool mode) or the process (lazy loading).
- G5 — **Stay boring and portable**: stdlib-first Go (1.23), no Docker
  runtime dependency, npm + Docker distribution channels.

### 1.2 Non-goals (explicitly out of scope)

- A GUI, dashboard, or SQL editor.
- A general-purpose database client; the tool surface is fixed and
  deliberate.
- Schema migration *engine* (DDL is possible via `execute`, but the server
  does not manage migration history).
- HA/clustering of the server itself (single process; connections pool per
  database).
- Query result **editing** — the server is query-first; masking and
  read-only take precedence over convenience.

---

## 2. Architecture

```
┌──────────────┐   MCP (stdio / SSE / streamable)   ┌──────────────────────┐
│ MCP client   │ ─────────────────────────────────▶ │ cmd/server (flags,   │
│ (Claude,     │                                    │ config, transports,  │
│  Cursor, ...)│                                    │ /health endpoint)    │
└──────────────┘                                    └──────────┬───────────┘
                                                               │
                                              internal/delivery/mcp
                                              ToolRegistry + ToolType
                                              factory (per-db and
                                              unified registration) ——┐
                                              handles routing/params  │
                                                               ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      internal/usecase (business logic)               │
│  database_usecase · explain · describe · health · performance ·      │
│  index_advisor · index_health · workload_advisor · db_health ·       │
│  sql_guard (write classification) · masking · audit · tx mgmt        │
└──────────────────────────────┬───────────────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  internal/repository · pkg/db (per-engine drivers + DSN builders)    │
│  pkg/dbtools (query analyzer, SQL issue detector, querybuilder)      │
│  pkg/db/timescale (hypertables, policies, continuous aggregates)     │
└──────────────────────────────┬───────────────────────────────────────┘
                               ▼
                MySQL · PostgreSQL · SQLite · Oracle
```

Layers follow clean architecture: `internal/delivery` (MCP surface) →
`internal/usecase` (rules) → `internal/repository` / `pkg/db` (engines).
A `ToolTypeFactory` generates the tool set; the same factory powers both
per-database registration (`query_<db_id>`) and unified registration
(`query` with a `database` parameter).

### 2.1 Registry of docs

| Doc | What it is |
|-----|-----------|
| `README.md` | Quick start: install, transports, config reference, per-tool docs, testing. The PRD is linked from it. |
| [docs/cycles/INDEX.md](cycles/INDEX.md) | 59 product-cycle narratives (research → ship → verify). Historical work log + competitive baseline; keep as the changelog-with-reasoning |
| [docs/design/column-masking-scoping.md](design/column-masking-scoping.md) | Design record: why masking v1 is name-based (no SQL parser) |
| `docs/TIMESCALEDB_*` | TimescaleDB integration PRD, functions reference, tool docs (mirrored in README) |
| `docs/TESTING.md`, `docs/ORACLE_*`, `docs/NPM_RELEASE_TESTING.md` | Historical runbooks — see [archive](#7-document-hygiene) |
| `docs/benchmark-token-efficiency.md` | Token-cost numbers behind unified mode |
| `CHANGELOG.md` | Release notes (v1.8.0 → v1.13.0) |

---

## 3. Current capabilities (shipped)

### 3.1 Tool surface

Per-database mode (default) registers, for each connection `N`:

| Tool | Does |
|------|------|
| `query_N` | SELECT / DML-returning query; tabular result, masking + max_rows applied |
| `execute_N` | DML + DDL; write-classified, blocked on read-only DBs |
| `transaction_N` | `begin` / `execute` / `commit` / `rollback` with server-side transactions (real, stored) |
| `performance_N` | Actions: `stats`, `slow_queries`, `engine_slow_queries`, `suggest`, `suggest_indexes`, `validate_suggestions` (hypopg), `workload_suggestions`, `index_health`, `db_health`, `reset` |
| `explain_N` | Execution plan (PostgreSQL/MySQL); `analyze: true` for timed runs; index advice appended |
| `describe_N` | One table: columns, indexes, row estimate, PK/FK constraints |
| `health_N` | Connectivity, pool pressure, engine stats, guardrail visibility |
| `schema_N` | Database schema; `format=mermaid` renders the FK relationship ERD |
| `list_databases` | Registered connection IDs (global) |
| `filter_tables` | Substring table-name search across databases (global) |

TimescaleDB adds seven discovery/query tools on PostgreSQL connections with
the extension (`timescaledb_timeseries_query`, `timescaledb_analyze_timeseries`,
`timescaledb_list_hypertables`, `timescaledb_compression_settings`,
`timescaledb_retention_policy`, `timescaledb_list_continuous_aggregates`,
`timescaledb_continuous_aggregate_info`) — auto-detected at startup (or by
config type under `--lazy-loading`), all read-only-safe.

**Unified mode** (`--unified-tools`, recommended at 5+ databases): the same
tool set once each, taking a required `database` parameter. Measured cost:
~1.25–1.6k tokens flat regardless of database count vs ~800 tokens/db in
per-database mode (≈80% wire saving at 10 DBs; see
[docs/benchmark-token-efficiency.md](benchmark-token-efficiency.md)).
Also resolves MCP-client tool-count limits (Claude — issue #18).

### 3.2 Engine coverage

| Engine | Connect | Read-only enforcement | Notes |
|--------|---------|----------------------|-------|
| PostgreSQL | ✓ | ✅ engine-level `default_transaction_read_only=on` (+ classifier) | pg_stat_statements, hypopg, TimescaleDB |
| TimescaleDB | (via PG) | ✅ inherited from PG | hypertables, compression, retention, continuous aggregates |
| MySQL | ✓ | ✅ engine-level `transaction_read_only=1` (+ classifier) | performance_schema digests |
| SQLite | ✓ | ✅ `mode=ro` (+ classifier) | file/encrypted/in-memory |
| Oracle | ✓ | ⚠️ classifier + least-privilege only — no engine session default | v$sqlarea stats — see gap G-1 |

### 3.3 Guardrails & governance

- **Read-only**: three layers — app classifier (`sql_guard` strips comments,
  detects stacked statements and data-modifying CTEs), engine session
  defaults (PG/MySQL), least-privilege users (all).
- **max_rows**: truncates results with an explicit `[Truncated]` notice;
  0 = unlimited.
- **query_timeout**: per-database seconds (default 30, `-1` disables);
  `QUERY_TIMEOUT_SECONDS` env fill for env-only deployments; propagated into
  MySQL DSN timeouts.
- **Masking** (`masking_rules`): name-based regex on output columns; v1
  strategies `fixed_string` / `null` / `partial` (keep_last); fail-closed
  validation; masked-cell counts. Design rationale in
  [docs/design/column-masking-scoping.md](design/column-masking-scoping.md).
- **Audit** (`DB_MCP_AUDIT_LOG`): append-only JSONL per executed statement
  (op, database, statement ≤10k chars, duration, error), includes rejected
  read-only attempts; 0600 file; best-effort writes.
- **Transports/auth**: stdio, SSE, streamable HTTP; bearer API-key
  middleware for HTTP transports (`--api-key` / `DB_MCP_API_KEY`).

### 3.4 Support surface

- Config: JSON file, inline `-db-config` / `DB_CONFIG`, `.env` + env vars,
  per-database fields for pool sizing, per-engine options, masking, guardrails.
- Distribution: Go binary (linux/macOS), Docker multi-arch image
  (`freepeak/db-mcp-server`), npm launcher package.
- CI: `go.yml` (build + unit + live-engine integration via Docker services),
  `docker-publish.yml` (multi-arch on push/tag), `npm-publish.yml`. The two
  publish workflows gate their credential check on the event — a tag push or
  manual dispatch fails fast without secrets, a branch push never attempts a
  login or push — so a code push to main stays green and only a real publish
  run is blocked on credentials.
- /health endpoint on a dedicated port (SSE mode) for orchestrators.

---

## 4. Roadmap (ordered)

Where the product goes next, highest value first. Owner-agnostic; each item
is sized against the cycle-doc process.

### 4.1 Release gate — DONE (v1.13.0 cut 2026-10-12)

**Status: landed.** The release gate is cleared:

1. **PR #88** (hardening: canonical config, strict validation, real
   transaction lifetimes, streamable HTTP auth, unprivileged container)
   merged 2026-10-11 after a rebase onto main; build/integration/lint/verify
   all green. It also fixed the publish red-runs by scoping the credential
   guards to tag/dispatch events, superseding PR #91.
2. **Tool resolution fix** landed as **PR #92** (superseding #89, whose fork
   branch could not run CI here): exact-name lookup before underscore-prefix
   split, plus the regression test #89 shipped without. Authorship preserved
   in the history; #89 closed with credit.
3. **v1.13.0** cut: CHANGELOG, `package.json` 1.13.0, tag + release
   workflows (cycle 60).

**Distribution status (2026-10-12).** Docker Hub is live: `DOCKERHUB_USERNAME`
and `DOCKERHUB_TOKEN` are set, and the channel now carries
`1.9.0`/`1.9`, `1.10.0`/`1.10`, `1.11.0`/`1.11`, `1.12.0`/`1.12`,
`1.13.0`/`1.13`, `latest` and a `sha-*` tag — all multi-arch (linux/amd64 +
linux/arm64) with provenance and SBOM attestations. The v1.13.0 tag push
(run 38199006601) moved `latest` off the stale 2025-05-09 build. GitHub
Release v1.13.0 carries all seven platform assets.

**Still open — issue #86 is now npm-only.** `NPM_TOKEN` is unset, so
`Release artifacts → Publish npm package` fails at its credential check by
design and `@freepeak/db-mcp-server` still 404s on the registry. Add a
granular token (Read & Write, `@freepeak` scope) as `NPM_TOKEN` under
Settings → Secrets and variables → Actions, then re-run that workflow with
version `1.13.0`.

### 4.2 Quick wins (do next, small surface)

- **Env-var doc currency** (backlog #10 re-check): **done 2026-10-12.**
  `VERBOSE` and `MCP_DISABLE_LOGGING` were read by the server and missing
  from the README env table; both are now documented. The item also named
  `API_KEY` and `HEALTH_PORT` — neither is read from the environment (the
  code reads `DB_MCP_API_KEY`, already documented, and the health port is a
  flag only), so documenting them would have been wrong.
- **Oracle engine read-only hardening (gap G-1)**: currently classifier-only;
  options: `ALTER SESSION SET READ ONLY` on pooled session (mirror the
  `newReadOnlySessionConnector` pattern used for Oracle today), or extend
  the fail-closed privilege audit. Closes the "four engines enforce at the
  engine" claim.
- **SQLite write-classification gap check**: **checked 2026-10-12 — the engine
  holds.** `mode=ro` refuses `PRAGMA user_version=`, `journal_mode=` and
  `incremental_vacuum` outright; `writable_schema=ON`, `optimize`,
  `auto_vacuum=`, `shrink_memory`, `wal_checkpoint()` and `vacuum` are
  *accepted in memory* but never reach disk (database file byte-identical
  after each). The classifier still lists `PRAGMA` as a read-only leading
  keyword, which is now correct; the residual wart is that a no-op PRAGMA
  reports success instead of an error. Pinned by
  `pkg/db/sqlite_readonly_pragma_test.go`.
- **Pre-commit hooks / golangci in CI**: the repo declares golangci-lint and
  pre-commit conventions (`.pre-commit-config`); wire them into `go.yml` so
  lint debt cannot regress.

### 4.3 Active terrain — decision needed on the hackathon branch

`origin/hackathon` (PR #87) carries ~66 more cycles' worth of features —
**verified NOT on main** (main = 120 Go files; branch = 396). The valuable,
read-only/safe subset worth porting forward:

| Feature (branch) | What it adds | Value |
|---|---|---|
| Column masking **content-based + enforced** (`mask_pii` config, Luhn cards) | Real PII protection beyond name-regex v1 | High |
| `dry_run` + risk advisories + `-risk-warn-at` | Offline statement risk analysis before execution | High |
| DELETE/UPDATE **snapshots + rollback_snapshot** | Undo for mutations done through the server | High |
| **Auto-LIMIT** injection (Oracle via ROWNUM wrap) | Default unbounded SELECTs get a server-side cap | Medium |
| **Verbosity modes** (minimal/normal truncation) | 60–95% context savings on results | Medium |
| `generate_schema` → Go/TS code; `format=csv/json/inserts` | Schema-to-code + data export | Medium |
| Query history ring + durable sink; session report | Observability | Medium |
| `list_sessions`, `lock_waits`, `cancel_query` | Live engine ops | Medium |
| Schema drift + `check_schema_drift`; cross-DB `schema format=compare` | Diff between states/DBs | Medium |
| The TARDIS pack (wal_level, xid wraparound...) | Postgres ops telemetry | Low — pickup on demand |

**Recommendation**: do not merge the branch wholesale — it is one giant diff
(28k insertions) that rewrites files main has since touched. Rebase PR #88
first, then port the high/medium read-only-safe items as small PRs, TDD
style, so each lands reviewable. The masking/dry-run/snapshot trio is the
differentiator cluster and the natural "next feature to build".

### 4.4 Vision list (from the cycle backlog, still valued)

- **Table-qualified masking** (backlog #11): policies like
  `mask orders.customer_id` need SELECT-target-to-table resolution — adopt
  a real SQL parser (vitess/pingcap) only after name-based value is proven
  in the field.
- **Metrics + observability** (from TODO registry): tool usage counters,
  per-tool latency, Prometheus-style endpoint; structured logs.
- **Tool-discovery mechanism** (from TODO registry): replace hardcoded
  tool-list arrays with a declarative registry (shrinks
  `tool_types.go`/`tool_registry.go` duplication).
- **Rate limiting + resource protection** (from TODO registry): concurrent
  query caps per database.
- **`workload_suggestions` engine parity**: currently PG/MySQL/Oracle
  ranking; extend composite synthesis depth.

---

## 5. Key decisions (kept with rationale)

| Decision | Rationale | When |
|---|---|---|
| Tool set is fixed & curated | Competitive research (MCP Toolbox): tool curation cuts context up to 30×; token-lean beats feature-bloated surface | cycles 04/12 |
| Read-only-by-default + dual guardrails (classifier + engine) | DBHub lesson: app checks alone get bypassed; enforce at the engine too | cycle 03 |
| Unified mode as a flag, per-db as default | Default matches small setups; flag rescues big ones + Claude tool-count limits (#18) | cycle 12 |
| Masking v1 = name/pattern-based, no SQL parser | Name-based rules apply to every query shape incl. `SELECT *`; parser deferred until table-qualified rules are needed | cycle 35 |
| Oracle read-only = classifier + privilege audit | go-ora has no DSN-level read-only switch; fail-closed audit refuses write-capable credentials | cycle 44 |
| Audit trail env-gated, JSONL, best-effort | Audit destination belongs to deployment config, not per-database JSON; never fail a query for audit | cycle 53 |
| stdlib-first, minimal deps | Boring, portable; Cortex MCP SDK + per-engine drivers only | throughout |

---

## 6. Milestones

| Milestone | Status |
|---|---|
| v1.10.0 — guardrails pack (#85): read-only bypass fix, max_rows, real transactions, explain/describe/performance, unified surface ancestor | ✅ shipped 2026-08-22 |
| v1.11.0 — distribution verification tag (Oracle support, SQLite, npm/docker automation) | ✅ shipped 2026-08-22 |
| v1.12.0 — governance: engine read-only for Oracle, masking, audit trail, hypopg-validated index advice, engine slow queries, health | ✅ shipped 2026-08-25 |
| **v1.13.0 — hardening (#88) + tool resolution (#92); Docker Hub publishing live** | ✅ shipped 2026-10-12 — npm still blocked on [#86](https://github.com/FreePeak/db-mcp-server/issues/86) |
| Governance suite (masking content, dry-run, snapshots) ported from hackathon branch | ⏳ backlog — see [4.3](#43-active-terrain--decision-needed-on-the-hackathon-branch) |

---

## 7. Document hygiene

- **This PRD is the single spec**: features, roadmap, decisions, milestones.
  Updates land in the same PR as the change that triggers them, and the
  `*Last updated*` stamp is bumped.
- **`README.md` stays the quick start**: install/run/config/reference.
  It links here for "where is this going".
- **`docs/cycles/` is the historical work log** — kept as-is (linked from
  §2.1), never rewritten retroactively.
- **Archive**: point-in-time runbooks (`ORACLE_STATUS.md`,
  `ORACLE_SETUP.md`, `ORACLE-VALIDATION.md`, `ORACLE-TEST-REPORT.md`,
  `NPM_RELEASE_TESTING.md`) and the superseded `README-old.md` move to
  `docs/archive/` where they stay linked/available without cluttering the
  current tree.
- **`docs/design/column-masking-scoping.md`** stays as the design record for
  the masking decision.

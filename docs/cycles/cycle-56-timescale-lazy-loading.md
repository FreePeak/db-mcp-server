# Cycle 56 — TimescaleDB tools under lazy loading

**Date:** 2026-08-25
**Status:** ✅ Shipped
**Theme:** Close the gap cycle 55 left open — TimescaleDB tool registration forced a connection at startup, defeating `--lazy-loading`.

## Objective

Cycle 55 wired `list_hypertables` and fixed its read-only SELECT path, but registration still probed `pg_extension` eagerly for every PostgreSQL database before deciding whether to register the three read-only TimescaleDB tools. With lazy loading enabled (`--lazy-loading`, recommended at 10+ databases), that probe established a connection per database during startup — exactly what the flag exists to avoid.

## Changes shipped

### 1. Registration without probing (`internal/delivery/mcp/tool_registry.go`)

Per-database and unified-mode TimescaleDB registration were restructured into a single `registerTimescaleTools` closure with two branches:

- **Eager mode** (unchanged behavior): probe `pg_extension`; only databases where the extension is actually installed get the tools. Keeps the token-lean surface for plain PostgreSQL.
- **Lazy mode**: register on config type alone (`type == "postgres"` / `"timescale"` / `"timescaledb"`), no connection. Handlers verify the extension at call time instead.

Unified mode follows the same split, registering once against the first PostgreSQL-configured database.

### 2. Runtime extension guard (`internal/delivery/mcp/timescale_tool.go`)

New `ensureTimescaleExtension` helper runs before the three wired read handlers (`list_hypertables`, `time_series_query`, `analyze_time_series`):

- Extension present → operation proceeds.
- Extension absent → actionable error: `enable it with CREATE EXTENSION timescaledb…`.
- Probe fails (connectivity/permissions) → fail-closed `cannot verify` error rather than a raw catalog error mid-query.

In `handleListHypertables` the engine-type check runs **before** the probe so non-PostgreSQL databases keep their friendlier "TimescaleDB operations are only supported on PostgreSQL databases" error.

## Verification

- `go build ./...`, `gofmt`, `go vet ./internal/delivery/mcp/` clean.
- Full unit suite green (`go test -count=1 -short ./...`).
- New tests:
  - `TestEnsureTimescaleExtension_Absent` — absent extension yields guidance mentioning `CREATE EXTENSION timescaledb`.
  - `TestEnsureTimescaleExtension_QueryError` — failed probe fails closed ("cannot verify").
  - `TestTimeSeriesQueryTool` subtests extended with a distinct pg_extension guard expectation (mutually-exclusive `MatchedBy` matchers).
  - `TestHandleListHypertables` expectations split into guard + catalog reads.

## Notes

- The guard's presence check is a substring test on rendered query output (`" 1"`) matching the existing `pg_stat_statements`/hypopg checks' convention; mock responses must render the count with a space (`[{"n": 1}]`) to satisfy it.
- Write-path operations (create hypertable, retention policies, policies) remain eager-registration-gated; they were not part of this cycle's scope since they were never registered lazily.

## Artifacts

- Commit: `feat(timescale): lazy-loading-safe registration + runtime extension guard`

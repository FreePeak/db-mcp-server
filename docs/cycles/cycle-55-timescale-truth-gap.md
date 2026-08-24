# Cycle 55: TimescaleDB truth gap — wire the read-only discovery tool

**Status**: ✅ Shipped
**Date**: 2026-08-25

## Objective

The README advertised seven TimescaleDB tools; only two were actually registered
(`timescaledb_timeseries_query_<db_id>`, `timescaledb_analyze_timeseries_<db_id>`).
A ~2k-line `timescale_tool.go` implements hypertable, compression, retention, and
continuous-aggregate operations that no MCP client could ever reach. This cycle
closes the most valuable part of the gap honestly: expose the read-only discovery
tool, fix a read-only-safety defect in the wired tools' execution path, and make
the documentation state exactly what is wired versus parked.

## Research findings

- `internal/delivery/mcp/timescale_tool.go` implements 16 operations behind one
  `operation` parameter, but `tool_registry.go` registers only the two time-series
  tools (per-db + unified), gated on a `SELECT 1 FROM pg_extension WHERE extname =
  'timescaledb'` probe and skipped entirely under `--lazy-loading`.
- `CreateListHypertablesTool` existed with a complete handler reading
  `_timescaledb_catalog.hypertable`/`dimension` (table name, schema, time column,
  dimensions, space partitioning) — implemented, never wired.
- No unified-mode creator existed for list-hypertables.
- **Defect found while wiring**: every handler executed through
  `ExecuteStatement`, including pure SELECTs (`list_hypertables`,
  `time_series_query`, `analyze_time_series`). On databases configured with
  `"read_only": true`, `ExecuteStatement` is blocked by the write classifier — so
  all three read-only tools would fail on exactly the deployments where read-only
  tools matter most.

## What shipped

1. **Read-only-safety fix** — the three SELECT-only handlers now call
   `ExecuteQuery` instead of `ExecuteStatement`, so they work on `read_only`
   databases. Write-policy handlers keep `ExecuteStatement` deliberately.
2. **Wired `timescaledb_list_hypertables_<db_id>`** per-database and
   **`timescaledb_list_hypertables`** in unified mode (new
   `CreateUnifiedListHypertablesTool`), both behind the existing extension probe.
3. **README truth pass** — the TimescaleDB tools table now lists the three real
   tools plus a scope note stating that write-policy operations (hypertable
   creation, compression, retention, continuous aggregates) are implemented but
   not exposed; supported-databases row updated to match.
4. Test expectations updated in three places to pin the ExecuteQuery behavior.

## Verification

- `go build ./...` clean; full `-short` unit suite green.
- Mock-based tests pin that list/query/analyze route through `ExecuteQuery`
  (read-only-safe) while write operations still require `ExecuteStatement`.

## Decision: leave write-policy tools unwired

Exposing `create_hypertable`, compression/retention policy changes, and
continuous-aggregate DDL as agent-facing tools would hand any connected model
schema-mutating power with no additional guardrail beyond the existing read_only
classifier. That is a governance decision (Bytebase-style approval flow territory),
not a plumbing task. Parked as backlog item; plain SQL via execute tools remains
the escape hatch for operators who want it today.

## Artifacts

- Commit: this repository, branch main
- Backlog addition: expose TimescaleDB write-policy tools behind explicit opt-in

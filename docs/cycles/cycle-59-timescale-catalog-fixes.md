# Cycle 59: TimescaleDB catalog fixes validated against a real engine

**Status:** ✅ Shipped
**Commit:** a89fd2e

## Objective

Cycle 58 registered seven read-only TimescaleDB operations, but their SQL was
written from memory against old catalog shapes. This cycle ran every read-only
operation against a real TimescaleDB container (the same live-validation
discipline as cycles 31/32/39) and fixed whatever broke.

## Findings

Five of seven operations returned errors or wrong columns on a modern engine.
Root causes, all the same class — pre-2.x catalog assumptions:

| Operation | Broken assumption | Fix |
|---|---|---|
| `list_hypertables` | `_timescaledb_catalog.dimension.column_type` is now an OID, not text | Read public `timescaledb_information.hypertables` |
| `get_compression_settings` | `segmentby`/`orderby` aggregate columns no longer exist | Per-column rows: `attname`, `segmentby_column_index`, `orderby_column_index` |
| `get_retention_policy` | `job_stats.schedule_interval` removed | Read `schedule_interval` from `jobs` view directly |
| `list_continuous_aggregates` | `source_table`/`bucket_interval`/`aggregations`/`refresh_policy` gone | Current stable names: `view_schema/view_name/materialized_only/compression_enabled/materialization_hypertable_name` |
| `get_continuous_aggregate_info` | same as above | same |

Lesson reinforced: **never trust catalog SQL that hasn't touched the engine it
targets.** The information views (`timescaledb_information.*`) are the stable
public contract; internal catalogs (`_timescaledb_catalog`) change between
releases without notice.

## What shipped

- Five SQL rewrites in `internal/delivery/mcp/timescale_tool.go`
- `internal/delivery/mcp/timescale_live_test.go`: `TestTimescaleReadOnly_Live`
  seeds a hypertable + continuous aggregate idempotently, exercises all seven
  read-only operations end-to-end, skips gracefully when the container on port
  15435 is unreachable.

## Verification

```
go test -count=1 -run TestTimescaleReadOnly_Live ./internal/delivery/mcp/
ok  github.com/FreePeak/db-mcp-server/internal/delivery/mcp
go build ./... && go test -short ./internal/...   # all packages ok
gofmt + golangci-lint pre-commit checks passed
```

## Follow-ups

- ~~Wire the timescaledb-test compose service into CI~~ — done in this cycle:
  a `timescale/timescaledb:latest-pg16` service (port 15435) joined the
  integration job and `TestTimescaleReadOnly_Live` runs in the live-engine
  regression step on every PR and main build.
- Dependabot flagged 1 low vulnerability on main — triage next cycle.

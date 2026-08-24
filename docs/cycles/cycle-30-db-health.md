# Cycle 30 — Unified `db_health` with Connection Pressure

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- New `db_health` performance action: the analyze_db_health-parity summary view. Everything `index_health` covers (structure findings, usage evidence, bloat) plus a connection-pressure section:
  - PostgreSQL: active/open counts from `pg_stat_activity` vs `current_setting('max_connections')`.
  - MySQL: `THREADS_CONNECTED` from performance_schema global_status vs global_variables `MAX_CONNECTIONS` (fetched as two separate catalog reads — queryTableMetadata returns only the first successful candidate's rows; the initial draft wrongly passed both queries as candidates).
  - Warning at ≥80% capacity pointing at idle-in-transaction sessions; below that it renders an observation line only.
  - SQLite/unknown engines: no connections section rather than fabricated data.
- Threshold math isolated in pure `formatConnectionReport` for unit testing.

## Verification
- Unit: 12% utilization renders observation without warning; 90% warns; unknown max renders nothing.
- SQLite e2e: db_health returns index-health content and no Connections section.
- Full suite green across all packages; live smoke passes; vet/gofmt clean.

## Fed Forward
- A live-PG gated regression test (performance_live_test.go pattern) would exercise pg_stat_activity end to end once the compose stack is available.
- Per-database breakdown on multi-DB instances of pg_stat_activity (currently scoped to current_database()).

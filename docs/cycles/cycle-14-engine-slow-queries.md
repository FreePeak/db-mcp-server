# Cycle 14 — Engine-Level Slow Queries (Ground Truth)

**Status:** ✅ Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Cycle 08's performance data is in-memory only — it dies with the server process and misses traffic from other clients. Postgres MCP Pro's depth comes from the database's own statistics.
- `pg_stat_statements` (PostgreSQL) and `performance_schema.events_statements_summary_by_digest` (MySQL) provide durable, cross-client statement stats.

## Shipped
- `engine_slow_queries` action on `performance_<db_id>`:
  - PostgreSQL/Timescale: top-N by mean_exec_time from pg_stat_statements, gated by an extension-presence check that returns enablement instructions when absent
  - MySQL: digest table top-N by total time (avg/total ms + call counts)
  - Unsupported engines (SQLite/Oracle): explicit capability note
- Failure notes now carry engine-specific remediation hints (learned live: compose's MySQL user lacks SELECT on performance_schema; PG needs shared_preload_libraries for tracking).

## Verification
- Live against compose stack: MySQL path exercised end-to-end (digests or grant-hint note); PG path proves graceful no-error degradation whether or not the extension/preload is present.
- SQLite unsupported-engine unit test; full suite + lint green.

## Fed Forward
Hypothetical-index tuning remains the last big Postgres-MCP-Pro gap; explain output + engine slow queries are its natural inputs.

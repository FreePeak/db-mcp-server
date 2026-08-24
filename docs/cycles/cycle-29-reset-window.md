# Cycle 29 — Observation-Window Precision + Doc Currency

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- UNUSED findings now state their observation window: when PostgreSQL reports a reset timestamp (`pg_stat_database.stats_reset`), the footer reads "statistics were last reset at <ts>", so zero-scan evidence is judgeable instead of ambiguous.
- `usageFindings` restructured to return a `usageStats` struct (findings + resetTS) — the catalog switch gained a `stats_reset` case alongside idx_scan/indisvalid/n_dead_tup/data_free.
- README performance-tool row documents the seventh action: `index_health`, with an honest scope description (catalog-driven structure + bloat; usage evidence where engine statistics exist).

## Verification
- Claims checked against index_health.go:90-98 (all five statistics queries present).
- Full suite green across all packages; live smoke passes; vet/gofmt clean.

## Fed Forward
- MySQL has no per-instance stats-reset timestamp in sys.schema_unused_indexes ("since server start" wording already covers it).
- Connection-pressure findings remain the last un-shipped db-health component; blocked on a live-PG test story.

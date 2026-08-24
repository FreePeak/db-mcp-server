# Cycle 28 — Table Bloat Findings + Repo Hygiene

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- Bloat/fragmentation findings complete the index_health picture toward analyze_db_health parity:
  - PostgreSQL: dead-tuple analysis from `pg_stat_user_tables` — flags tables with ≥1000 dead tuples AND ≥20% dead ratio, suggesting `VACUUM (ANALYZE)`. The dual gate keeps tiny tables quiet and avoids flagging healthy high-churn tables.
  - MySQL: `information_schema.DATA_FREE` > 16 MB reported as FRAGMENTATION candidates for OPTIMIZE TABLE, with an honest note that DATA_FREE is a coarse per-tablespace signal.
  - SQLite: nothing to report (manual whole-db VACUUM); footer disclosure unchanged.
- Numeric coercion helper (`rowInt`) handles int64/int/float64 cell shapes across drivers.
- `.gitignore`: local build binary, data/, *.db, test_sqlite no longer pollute git status.
- `config.test.json` committed — scripts/smoke.sh depends on it and it contains no secrets (a SQLite path).

## Verification
- TestBloatFindings locks in the ratio/floor gates (83% flagged, 900-dead small table skipped, 1% ratio skipped) and MySQL byte-to-MB rendering with engine tag.
- Full suite green across all packages; live smoke passes; vet/gofmt clean.

## Fed Forward
- Surface pg_stat_database.stats_reset so UNUSED claims can state their observation window.
- Connection-pressure findings (pg_stat_activity saturation vs max_connections) would complete the db-health trio; needs a live-PG test story first.

# Cycle 27 — Usage-Evidence Findings in Index Health

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `index_health` now consults engine statistics catalogs where they exist, closing the gap its own cycle-26 caveat admitted:
  - PostgreSQL: never-scanned indexes (`pg_stat_user_indexes.idx_scan = 0`) and invalid indexes (`pg_index.indisvalid = false`, typically failed CREATE INDEX CONCURRENTLY leftovers).
  - MySQL: never-read indexes via `sys.schema_unused_indexes`.
  - SQLite/unknown engines: no statistics catalogs — the report renders without usage evidence and the footer discloses that explicitly.
- Footer differentiates honestly between the two modes: with usage stats it warns that zero scans since reset/start can be legitimate for young indexes; without them it states the view ran blind.

## Verification
- Formatter unit tests lock in all three row shapes (pg unused, pg invalid, MySQL sys) including engine-appropriate drop syntax.
- SQLite e2e: clean path returns no-findings; with a duplicate present the finding renders alongside the "ran without them" footer.
- Full suite green across all packages; live smoke passes; vet/gofmt clean.

## Fed Forward
- Reset-time awareness: pg_stat_user_indexes has stats_reset in pg_stat_database — surfacing "statistics last reset" would make UNUSED claims more precise.
- Table bloat estimation remains the un-shipped half of analyze_db_health parity; requires engine-specific free-space heuristics.

# Cycle 26 — Index Health Analysis (`index_health`)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Postgres MCP Pro's remaining headline capability is `analyze_db_health` (unused/invalid/redundant indexes, bloat). Our catalog plumbing from cycles 14–19 covers the index half engine-agnostically; bloat and unused-index evidence require engine statistics (pg_stat_user_indexes, sys.schema_unused_indexes) — deferred with an explicit caveat in the output rather than faked.

## Shipped
- New `index_health` performance action (`IndexHealth`, internal/usecase/index_health.go): enumerates user tables per engine, parses each table's indexes from the same catalogs the advisor uses (SQLite/PostgreSQL CREATE INDEX definitions, MySQL grouped SHOW INDEX rows), and reports:
  - DUPLICATE findings: exact same columns + uniqueness (group canonicalized to the alphabetically smallest name).
  - REDUNDANT findings: one index's columns are a leading prefix of another's and offer no stronger guarantee.
  - Engine-appropriate DROP syntax (MySQL: ALTER TABLE ... DROP INDEX; others: DROP INDEX).
- Output carries an honest caveat that usage-based evidence is not consulted.

## Design Lesson (caught by tests)
- Naive pairwise analysis produced contradictory advice on three overlapping indexes ("DUPLICATE: keep X" then "REDUNDANT: drop X"). Fix: canonicalize duplicate groups to one keeper first, run prefix-redundancy only over the canonical set — verdicts stay consistent (keep the widest index, drop everything it subsumes).
- Second catch: a name-tiebreak guard on the directional prefix check broke when the larger index's name sorted first alphabetically (`_` < letters). The guard was only needed for symmetric pairs; duplicates dedupe via canonicalization now.
- MySQL row-order robustness: multi-row SHOW INDEX groups re-sort columns by Seq_in_index; locked in by unit test.

## Verification
- SQLite e2e: duplicate pair + redundant prefix pair both found, consistent keep/drop verdict asserted; clean database returns the no-findings path.
- Unit: MySQL Seq_in_index grouping; uniqueness guard (unique-under-non-unique never flagged redundant).
- Full suite green across all packages; live smoke passes; vet/gofmt clean.

## Fed Forward
- Usage-aware findings when engine statistics are available (pg_stat_user_indexes idx_scan counts; MySQL sys.schema_unused_indexes) — read-only catalog queries fit the existing pattern.
- Invalid-index detection for PostgreSQL (pg_index.indisvalid = false) — needs a different catalog query shape (pg_index join pg_class), not just indexQueries.

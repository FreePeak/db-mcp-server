# Cycle 42 — Constraint-Aware Index Coverage (backlog #4 closeout)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Closes backlog #4's last remaining piece:
- New `constraintCoveredCols` helper: reads each engine's constraint catalog via the existing `constraintQueries` plumbing and returns columns enforced unique by PRIMARY KEY or UNIQUE constraints.
- Both suggestion paths in `emitIndexSuggestions` now treat those columns as covered — composite candidates whose leading column is constraint-backed are dropped, and single-column suggestions skip constraint-covered columns.
- SQLite gap filled in `constraintQueries`: UNIQUE constraints materialize as autoindexes with NULL sql, invisible to sqlite_master-based index listings; a new `pragma_index_list ... WHERE origin = 'u'` candidate query surfaces them.

## Design Notes
- TDD: the failing test was written first and exposed both gaps exactly as predicted (UNIQUE column suggested, PK column suggested).
- Failure semantics stay consistent with the advisor's style: if the constraint catalog is unreadable, suggest as before rather than go blind.

## Verification
- New test locks in both directions on real SQLite (`token TEXT UNIQUE` and `id INTEGER PRIMARY KEY` queries produce no CREATE INDEX).
- Full advisor suite green (9 tests); full suite green uncached; smoke passes; vet/gofmt clean.

## Backlog Impact
- Backlog #4 fully done: heuristic advisor + composites (17–18) + workload-driven analysis (19/23/40) + constraint-aware coverage (42).

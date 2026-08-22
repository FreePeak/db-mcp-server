# Cycle 20 — Index Advisor v2: Composite Suggestions + PK Awareness

**Status:** ✅ Shipped · **Branch:** hackathon

## Research Findings
- Postgres MCP Pro's hypothetical-index tuning models multi-column indexes; cycle 17's advisor emitted one index per column, producing redundant DDL for AND-filtered queries.
- Cycle 17 fed forward two gaps: composite grouping and PK re-suggestion noise. Constraint catalogs from cycle 9 (`constraintQueries`) already carried the data needed to close both.

## Shipped
- **Composite grouping:** ≥2 uncovered WHERE columns on one table → single `CREATE INDEX idx_t_a_b ON t (a, b)` replacing N singles. Join/sort columns stay single-column candidates.
- **PK suppression:** constraint catalog consulted per table; primary-key columns filtered before suggestion (fixes SQLite `TEXT PRIMARY KEY` and Postgres PK noise flagged in cycle 17).
- Candidate provenance (`where`/`join`/`sort`) drives grouping; deterministic output order retained.

## Verification
- TDD RED-first: composite test failed on v1 output (two singles), PK test failed (`idx_products_sku` suggested for a TEXT PRIMARY KEY). Both green after implementation; original cycle-17 suite unchanged — no regression.
- Full suite (9 packages), vet, golangci-lint clean (removed dead helper found by linter). Zero Docker.

## Fed Forward
- Partial-cover extension (existing `(a)`, query filters a+b) could suggest `ALTER`/recreate with `(a, b)` explicitly labelled as an extension.
- Luhn check for card masking (cycle 19 carry-over) remains open.

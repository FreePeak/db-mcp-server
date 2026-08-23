# Cycle 17 — Index Advisor (suggest_indexes)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Postgres MCP Pro's differentiator (per cycle-12 survey) is hypothetical-index tuning: tell the agent which indexes a slow query is missing. Our performance tool could list slow queries but said nothing about how to fix them.
- Competitor pattern worth copying without the cost: they run real `hypopg` simulations. A catalog-comparison heuristic ships on every engine we support with zero extensions — the right first move for a multi-engine server.
- While wiring the action, the performance tool's schema descriptions were found stale: they advertised actions that no longer exist (`getSlowQueries`, `analyzeQuery`, `setThreshold`), actively misleading agent callers.

## Shipped
- `performance` tool gains `suggest_indexes` action (`DatabaseUseCase.SuggestIndexes`, `internal/usecase/index_advisor.go`): extracts join / filter / sort columns from an ad-hoc query via identifier regexes, resolves table aliases to real tables, drops references that are not actual columns of their table (catalog check), and emits `CREATE INDEX idx_<table>_<col>` statements only where no existing index covers the column. Output labels itself heuristic and says verify with EXPLAIN.
- Coverage check matches whole identifier tokens, not substrings — an index on `surname` does not mark `name` as covered.
- Stale schema descriptions fixed across both per-db and unified performance tools; action list and parameter docs now match the real API.

## Verification
- Six tests in `internal/usecase/index_advisor_test.go`: e2e suggestion of unindexed filter columns, suppression when covered, substring confusion guard, non-column alias dropped with disclosure note, alias-resolved join columns across two tables, input guards (empty query, no tables).
- Two real defects caught red by those tests and fixed before push: alias names leaking into suggestions (`CREATE INDEX idx_b_author_id ON b (...)`) and `WHERE t.col =` attributing the qualifier instead of the column.
- Full suite + vet green.

## Known Limitations (fed forward)
- SQLite's index catalog hides primary keys (only explicit indexes appear), so PK join keys can be suggested; acceptable noise under the EXPLAIN-first contract, but constraint-aware coverage would remove it uniformly.
- Single-column suggestions only; composite-index synthesis from multi-column ORDER BY remains future work (Postgres MCP Pro territory).

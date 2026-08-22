# Cycle 17 — Index Advisor (`suggest_indexes`)

**Status:** ✅ Shipped · **Branch:** hackathon (worktree)

## Research Findings
- Postgres MCP Pro's differentiator is plan analysis + hypothetical-index tuning; DBHub has nothing comparable.
- Backlog item #4 ("Hypothetical-index tuning depth") was the highest-leverage next step after the ERD cycle.
- Prior session left `index_advisor.go` uncommitted with zero tests — TDD debt that this cycle repaid first.

## Shipped
- `performance_<db_id>` gains action `suggest_indexes` (+ unified variant): parses a SQL query's JOIN / WHERE / ORDER BY / GROUP BY columns, diffs against live catalog indexes (`pg_indexes` / `SHOW INDEX` / `sqlite_master`), and emits `CREATE INDEX` DDL labelled heuristic with an EXPLAIN-verify caveat.
- **Alias resolution (bug found by TDD):** `JOIN books b ON ...` previously produced `ON b (author_id)` — indexing the alias. New `extractAliases` maps aliases to physical tables; suggestions now always target real tables.
- Tool schema descriptions updated in both per-db and unified performance tools.

## Verification
- 6 new tests (RED first): e2e missing-index detection on real SQLite, all-covered → "(none)", single-table WHERE+ORDER BY, empty-query validation, non-SELECT guidance, and AnalyzePerformance dispatch wiring.
- Full suite, `go vet`, golangci-lint green. No Docker used anywhere.

## Fed Forward
- Suggestions ignore PK columns (SQLite rowid PKs produce noise-free output anyway, but Postgres PKs may be re-suggested); consult constraint catalogs next.
- Composite/multi-column suggestions (e.g. `(status, created_at)`) are one column per suggestion today.

# Cycle 35 — Column Masking Scoping (Research Phase)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Cycle 21 deferred column masking because "SELECT-target-to-table resolution requires real SQL parsing." That constraint only binds for **table-qualified** policies. **Name-based** policies need no parsing at all: `domain.Rows.Columns()` is already available inside `ExecuteQuery` before `formatQueryResults` renders text, so rules like "mask any output column matching /email|phone|ssn/" apply to every query shape — explicit lists, aliases, joins, and even `SELECT *`.
- Existing "mask" code in the repo is DSN password masking only; no result-set masking exists.
- Enforcement point confirmed: internal/usecase/database_usecase.go:260 → :305.

## Shipped
- [docs/design/column-masking-scoping.md](../design/column-masking-scoping.md): problem statement, the un-blocking insight, config schema sketch (per-database `masking_rules` mirroring `read_only`/`max_rows` precedent), v1 strategies (`fixed_string`, `null`; `partial`/`hash` deferred), enforcement point, the alias-renaming limitation stated rather than hidden, a three-cycle implementation breakdown, non-goals (audit/identity/write-path), and risks.
- Backlog #5 updated to point at the scoping doc.

## Verification
- Docs-only cycle; full suite green uncached, smoke passes, vet/gofmt clean.

## Fed Forward
- Cycle A of the breakdown is now unblocked: config schema + matcher + fixed_string/null strategies + SQLite e2e test.

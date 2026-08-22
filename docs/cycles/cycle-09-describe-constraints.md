# Cycle 09 — Constraints Surfacing in Describe

**Status:** ✅ Shipped · **Artifacts:** PR #85 (this cycle)

## Research Findings
- Cycle 07 exposed columns/indexes but not constraints — PK/FK knowledge is what lets agents write valid joins and understand relationships without trial queries.
- SQLite needs two complementary pragma queries (PK from `pragma_table_info`, FK from `pragma_foreign_key_list`) rather than fallback alternatives, so catalog-query execution semantics changed to accumulate-all-successes.

## Shipped
- `DescribeTable` now returns a normalized `constraints` set (constraint_name / constraint_type / column_name):
  - PostgreSQL/Timescale + MySQL: `information_schema` table_constraints ⋈ key_column_usage
  - SQLite: synthesized PRIMARY KEY rows + FOREIGN KEY rows via pragma table functions
  - Oracle: user_constraints ⋈ user_cons_columns
- Constraint introspection is best-effort: failures log a warning and return an empty set instead of failing the describe.
- Tool output renders a Constraints section.

## Verification
- SQLite e2e extended: PRIMARY KEY detected on the id column of a real table.
- Full suite + golangci-lint green.

## Fed Forward
Referenced-table/column detail for FKs could enrich relationship graphs later; schema-wide (multi-table) relationship map is a bigger follow-up.

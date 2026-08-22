# Cycle 15 — FK Referenced-Table Resolution

**Status:** ✅ Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Cycle 09's constraints output named the local FK column but not its target — agents had to guess join targets or run trial queries. Relationship closure (`books.author_id -> authors(id)`) is what turns schema inspection into correct join generation.

## Shipped
- Constraint catalog queries now return `referenced_table` / `referenced_column`:
  - PostgreSQL/Timescale: LEFT JOIN onto `information_schema.constraint_column_usage`
  - MySQL: `KEY_COLUMN_USAGE.REFERENCED_TABLE_NAME` / `..._COLUMN_NAME` in the existing join
  - SQLite: `pragma_foreign_key_list` provides parent table + parent column directly
  - Oracle unchanged (best-effort)
- Describe output renders references as `FOREIGN KEY <name> (<col>) -> <table>(<col>)`.

## Verification
- New e2e on real in-memory SQLite with a two-table FK schema asserts the exact author_id -> authors(id) resolution.
- Full suite + lint green.

## Fed Forward
With columns + PKs + resolved FKs available, a schema-wide relationship graph (tables as nodes, FKs as edges) is now cheap to build — candidate for agent-friendly ERD output.

# Cycle 32 — Live MySQL Validation of Catalog SQL

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Cycle 31 validated the PostgreSQL half; the mysql formula turned out to ship a full server (`mysqld` 9.7.1), so the same treatment was possible for MySQL. A throwaway instance on port 13306 with compose-matching credentials makes the live-gated pattern fully runnable locally for both engines.

## Bugs Found and Fixed (both by live execution)
1. **`sys.schema_unused_indexes` column names were wrong.** The query selected `table_name`, which doesn't exist — the view exposes `object_schema`/`object_name`/`index_name`. The graceful-skip meant unused-index findings silently vanished on every real MySQL. Fixed to `SELECT object_name AS table_name, index_name ... WHERE object_schema = DATABASE()`.
2. **My first fix was itself wrong and live data caught it:** aliasing `object_schema AS table_name` produced the database name (`UNUSED on db1:`) instead of the table. Correct source is `object_name`. Lesson: aliasing a wrong-but-existing column fails quietly in exactly the way graceful degradation hides.
- Also confirmed live: digest query (`events_statements_summary_by_digest`) executes with correct timer math (AVG_TIMER_WAIT/1e9 → ms); connection-pressure CASTs work on 9.7.

## Shipped
- `TestDbHealth_LiveMySQL`: asserts DUPLICATE findings from SHOW INDEX parsing, correct table attribution in UNUSED (no "db1" leakage), and that PRIMARY never receives DROP advice. Skips gracefully when unreachable.
- Recursive-CTE seeding needs `SET SESSION cte_max_recursion_depth = 5000` beyond ~1000 rows — noted here for future seed scripts.

## Verification
- Live run against real MySQL 9.7.1: all assertions pass; PG subtests skip cleanly (instance already torn down).
- Full suite green across all packages; smoke passes; vet/gofmt clean. Throwaway datadir removed.

## Fed Forward
- Both engines' catalog SQL now has a repeatable local validation path (initdb / mysqld --initialize-insecure + compose ports). Consider scripting it as scripts/live-db-setup.sh so future catalog changes are one command away from real-engine proof.

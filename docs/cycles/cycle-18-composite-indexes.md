# Cycle 18 — Composite Index Synthesis

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Competitor deep-dive (crystaldba/postgres-mcp): their index tuner explores thousands of candidate indexes but still emits only one permutation per multi-column index to bound search cost; column order follows the btree usability rule from pganalyze's cost-model writeup — equality columns first, then one inequality, then sort columns. A deterministic equality-first heuristic is therefore industry-defensible without any simulation.
- Gap confirmed in our cycle-17 advisor: `WHERE a=? AND b=? ORDER BY c` produced three separate single-column suggestions where competitor-grade output is one composite `(a,b,c)`.

## Shipped
- Predicate classification in `SuggestIndexes`: WHERE operators split into equality class (=, IN, LIKE/ILIKE) and range class (>, <, >=, <=, BETWEEN).
- Per-table composite synthesis: equality columns in appearance order followed by ORDER BY / GROUP BY columns, capped at three; emitted only when the leading column is not already indexed (leading-column rule). Composite members are suppressed as singles.
- Range predicates and join keys never enter composites (no equality prefix to lead them); they remain single-column suggestions.
- Removed dead helper (`orderedKeys`) left from the flat-candidate design.

## Verification
- Eight advisor tests: composite folding of two equality predicates (with no-singles assertion), equality-then-sort ordering `(tenant_id, created_at)`, pure-range queries staying single-column, plus all six cycle-17 behaviors re-verified.
- One red-test iteration: the first range test wrongly asserted that `ORDER BY price` must not pull price into a composite after an equality prefix — it must, that is exactly when a composite serves the sort. Test corrected to the real invariant (range-only queries produce no composite); implementation was already right.
- Full suite + vet + gofmt green.

## Fed Forward
- Workload-driven tuning: `engine_slow_queries` already surfaces statements; feeding its top offenders into SuggestIndexes would be our version of Postgres MCP Pro's `analyze_workload_indexes` (backlog #8).
- LIKE patterns without leading wildcard could be verified against the literal before eq-classing.

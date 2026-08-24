# Cycle 23 — Execution-Weighted Workload Suggestions

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Cycle 19's `workload_suggestions` counted distinct statements; a predicate hit 5x as often ranked the same as one hit once. Postgres MCP Pro tunes by resource consumption — real traffic weighting was the missing half of that parity (backlog #8 remainder).
- Data was already available: tracker metrics carry `Count`; pg_stat_statements has `calls`; MySQL digests have `COUNT_STAR`. Only plumbing was needed.

## Shipped
- `workloadStatements` now returns weighted statements: engine catalogs select execution counts alongside text; tracker fallback uses metric counts.
- Emitter rework (`emitIndexSuggestions`):
  - Composite candidates form **within a single statement only**. Cross-statement merging previously folded columns from disjoint predicates into one composite whose trailing columns served nothing — caught red by the new ranking test and fixed at the design level, not patched.
  - Each suggestion's weight counts every analyzed execution whose eq/sort columns it serves (subset semantics), so an index leading with a hot column credits the hot traffic.
  - Composite candidates rank by served executions before emission; single-column lists rank by weight within their class.
  - Header reports total executions analyzed; annotations read `serves N of M execution(s)`.
- Removed dead helper (`hitCols`) left from the earlier merge design.

## Verification
- Ranking test: `hot` filtered 5x vs `cold` 1x asserts hot is suggested first.
- E2E updated to execution semantics: three identical selects plus one ORDER BY variant total four executions, all served by the `(customer_id, state)` composite.
- All cycle 17–19 advisor behaviors re-verified green; full suite across all packages passes; vet/gofmt clean.

## Fed Forward
- Duration-weighted ranking (total time = avg_duration x count) may beat raw count for prioritization; needs a benchmark against realistic mixed workloads before switching.
- Engine digest tables normalize literals, so parameterized variants of one query shape collapse into one row — weighting reflects statement shapes, not literal values. Acceptable for index advice.

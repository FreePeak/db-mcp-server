# Cycle 40 — Duration-Weighted Workload Ranking (backlog #8 refinement)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Completes backlog #8's "duration-weighted ranking is the possible refinement":
- `weightedStatement` now carries `totalMillis`. Engine catalogs supply it directly (PostgreSQL `pg_stat_statements.total_exec_time`; MySQL `SUM_TIMER_WAIT/1e6`); the tracker fallback computes `AvgDuration × Count`.
- Weight selection prefers engine-reported total time when present — one slow query can now outweigh thousands of cheap ones, which traffic ranking got backwards. Falls back to execution counts for catalogs that omit durations.
- Output honestly discloses its basis: header says `ranked by estimated total time` vs `ranked by traffic`, and coverage annotations switch units (`serves X of Y ms of engine time` vs `execution(s)`) via a new `weightUnit` parameter on `emitIndexSuggestions` — milliseconds are never mislabeled as executions.
- Tracker fallback also sorts by total time (was mean duration), and smoke script assertions updated to the new wording.

## Design Notes
- `AvgDuration` is a `time.Duration`, not float64 — caught by build; millisecond conversion happens once at the boundary.
- Exact ms values in tests are nondeterministic (real timing), so tests assert stable wording rather than numbers.

## Verification
- All workload advisor tests green including end-to-end (duration-ranked header + time-unit annotation) and ranking order.
- Full suite green uncached; smoke passes over live stdio with updated assertions; vet/gofmt clean.

## Backlog Impact
- Backlog #8 fully done (heuristic advisor + composites + workload-driven analysis + duration weighting).

# Cycle 52 — Oracle Statement Statistics + Digest-Ranking Test Hardening

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- **Oracle joins the statement-statistics tier**: `engine_slow_queries` and `workload_suggestions` now read `v$sqlarea` (FETCH FIRST pagination, microsecond `elapsed_time` → ms). Without the grant, the action degrades to an actionable hint pointing at `scripts/oracle-init/03-grant-statement-stats.sql`. Statement coverage now spans PostgreSQL, MySQL, Oracle; SQLite honestly reports no catalog.
- Our own `v$sqlarea` read is excluded from index advice alongside the other system catalogs.
- Live test against real Oracle verified the granted data path (380 digest rows readable by testuser).

## Flaky Test Root-Caused and Fixed (TestEngineSlowQueries_IndexAdvice_Live)
The MySQL digest table is **cumulative since server start**, so the old warmup (two cheap executions) silently lost its top-5 slot as DDL history accumulated across full-suite runs — a latent race with repo hygiene, not product code. Three failed hardening attempts each taught something:
1. PK equi-join × 25 — too cheap (~sub-ms) to outrank 165ms of accumulated history.
2. Three-way cross join — MySQL ran it between **0.01s and 184s** depending on whether the optimizer noticed our filter matched zero rows (`tenant_id = 42` vs seeded values 0–6: impossible-WHERE short-circuit) or actually nested-looped millions of combinations.
3. Final form: `SELECT SLEEP(0.005)` per matching row — deterministic ~215ms/call, two calls dominate regardless of history.

## Verification
- Full suite green uncached; smoke green over live stdio.
- Flaky test passes 3× consecutively against the long-lived container.

## Design Notes
- MySQL prohibits granting write privileges on performance_schema tables entirely (root itself is denied), so digest-reset-based test isolation is not available — dominance-by-cost is the only robust pattern for cumulative-catalog tests.

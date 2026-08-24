# Cycle 46 — Slow-Query Index Advice (backlog #9 full closeout)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `engine_slow_queries` now appends bounded index suggestions derived from the statements it ranked — latency-ranked (per-call pain), distinct from `workload_suggestions`' total-time ranking, so the two views can legitimately surface different statements.
- Catalog-read multiplication concern from the cycle 38 deferral resolved: statement texts come from the `workloadStatements` path already used by the workload advisor; per-table index reads are limited to the distinct tables among the top few statements; advice is best-effort and never masks a slow-query failure.
- Two real bugs found by live validation:
  1. MySQL statement digests quote identifiers in backticks (`FROM \`orders\``), making every MySQL workload/slow-query statement invisible to the advisor's token matchers. Fixed centrally in `extractIndexAdvice` — benefits `workload_suggestions` too.
  2. The advisor's own catalog queries appear in digest statistics and would receive bogus suggestions on system tables; system-schema statements are now excluded from the advice path.
- Live-engine setup: `seed_my` now provisions `user1@'%'` — tests connect over TCP from the host, which previously fell outside the localhost-only grants (this was silently masking digest-table reads as graceful degradation).

## Verification
- New live test passes against real MySQL 8 (digest → advice end-to-end); new unit test locks backtick normalization; full suite green uncached; smoke green over live stdio; gofmt/vet clean.

## Backlog Impact
- Backlog #9 fully done — both explain (cycle 38) and slow-queries (this cycle) carry advisor wiring.

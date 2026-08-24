# Cycle 19 — Workload-Driven Index Suggestions

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Postgres MCP Pro's second headline capability is `analyze_workload_indexes`: tune from the live workload instead of one ad-hoc query. Our cycle-14 `engine_slow_queries` already reads the same catalogs (pg_stat_statements, MySQL digest tables) — but its display path truncates statements to 80 chars, which cuts off WHERE clauses and makes them unusable as advisor input.
- Design decision: engine catalogs preferred with full statement text; this server's own tracked history (all engines, including SQLite) as fallback. Competitor caps batch size at 10 queries — adopted (clamp 1–25, default 10).
- Digests/normalized literals (`$1`, `?`) are harmless to the advisor since it extracts column names only.

## Shipped
- New `workload_suggestions` performance action (`WorkloadIndexSuggestions`, `internal/usecase/workload_advisor.go`): pulls top-N expensive statements, runs each through the shared advisor core, merges candidates per table/class, and emits suggestions annotated `-- serves N of M statement(s)` so the agent can prioritize by coverage.
- Advisor refactored into reusable halves: pure `extractIndexAdvice(query)` (string → per-table predicate classes) and `emitIndexSuggestions(...)` (catalog comparison + rendering). `SuggestIndexes` is now a thin wrapper.
- Hardening found by tests: columns resolving outside a statement's extracted tables are dropped (previously an UPDATE without FROM created phantom empty-table candidates); alias resolution happens before the table-membership guard.

## Verification
- E2E: seed tracker via TrackQuery against in-memory SQLite (pinned to one pooled connection — `:memory:` gives each connection its own database, and an unclosed result set deadlocked the advisor's catalog queries on first run; both caught red), assert merged composite suggestion with distinct-statement coverage annotation.
- Unit: extractor returns nothing for INSERT/UPDATE-without-FROM. All 11 advisor tests plus full suite green; vet/gofmt clean.

## Fed Forward
- Weight suggestions by execution count (tracker metrics carry Count; engine digests carry execution totals) rather than distinct statements for sharper prioritization.
- Composite ORDER BY of multiple sort columns could synthesize wider composites when equality prefix is short.

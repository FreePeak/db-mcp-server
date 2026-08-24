# Cycle 24 — Live Protocol Smoke Test (Three Bugs Found)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- After seven feature cycles landed on unit/e2e harnesses, the highest-leverage step was in-vivo verification: drive the real binary over the MCP stdio protocol against config.test.json's SQLite instance.
- The smoke run immediately invalidated two assumptions the harnesses had baked in. Lesson reinforced: fake-driven tests verify logic; only protocol-level runs verify wiring.

## Bugs Found and Fixed
1. **In-process tracker never saw production traffic.** `usecase.ExecuteQuery` called `db.Query` directly, bypassing `dbtools.ExecuteQuery` where `TrackQuery` lives — so `slow_queries`, `stats`, and the workload-suggestions fallback were dead paths on the primary route. Fixed by wrapping execution in `TrackQuery` inside `ExecuteQuery` (rows closed within the closure).
2. **Health formatter dropped new guardrail keys.** Cycle 21 added `read_only` / `max_rows` / `statement_timeout_seconds` to the use-case map, but delivery-layer `formatHealthResult` whitelists keys and silently omitted them. Formatter extended; omission-of-unset behavior preserved.
3. **Adapter did not expose `QueryTimeout()` upward.** Unlike `MaxRows()`, there was no pass-through, so health checks could never observe the configured timeout even with the formatter fixed. Added `DatabaseAdapter.QueryTimeout()` mirroring the MaxRows pattern.

## Verification
- Live stdio session: three tracked queries then `workload_suggestions` reports "1 statement(s) (3 executions)" with correct weighting and serves annotation.
- Live `health` output now shows `read_only: false` and `statement_timeout_seconds: 30`.
- New regression tests: `TestFormatHealthResult_RendersGuardrails`, `TestFormatHealthResult_OmitsZeroValuedGuardrails`, `TestDatabaseAdapter_QueryTimeoutExposed`; full suite green across all packages.

## Fed Forward
- Add a scripted smoke check (stdio JSON-RPC script) runnable pre-release so protocol wiring regressions surface before push rather than during manual testing.
- ExecuteStatement path should get the same TrackQuery treatment for statement traffic visibility.

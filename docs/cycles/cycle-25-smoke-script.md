# Cycle 25 — Statement Tracking + Reusable Smoke Script

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `ExecuteStatement` now wraps execution in `TrackQuery`, matching the cycle-24 fix on the query path — statement traffic (INSERT/UPDATE/DELETE) is visible to `slow_queries`/`stats`/workload fallback too. Read-only enforcement still runs before tracking, so rejected writes never pollute metrics.
- `scripts/smoke.sh`: protocol-level smoke check driving a live stdio JSON-RPC session against config.test.json's SQLite instance. Asserts advisor output, tracker wiring via execution weighting, and health guardrail visibility — the three things unit harnesses structurally cannot see (per cycle 24's lesson). Configurable via `SMOKE_CONFIG` / `SMOKE_DB_ID`. Run before tagging a release.

## Verification
- Smoke script passes end-to-end on first run: `SMOKE OK: advisor, tracker wiring, and health guardrails all verified over live stdio protocol`.
- Full suite green across all packages; vet/gofmt clean.

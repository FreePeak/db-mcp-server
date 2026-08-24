# Cycle 48 — CI Green: Digest-Table Grant for Live Advice Tests

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `TestEngineSlowQueries_IndexAdvice_Live` failed in CI while passing locally: the MySQL service container grants `user1` access to `db1` only, so digest reads hit `performance_schema.events_statements_summary_by_digest` were denied and the advice path degraded gracefully (as designed) — leaving nothing for the test to assert.
- Fix: seed step grants `SELECT` on the digest table to `user1@'%'`, mirroring what cycle 46 put into `scripts/live-db-setup.sh`.
- Verified along the way that the docker/NPM workflow failures on the same commits are the known #86 secret-block (cycle 11's fail-fast guards firing correctly), not regressions.

## Verification
- Go Build & Test green on main after the fix (run for 74fd8bf), including all live-engine regression tests.

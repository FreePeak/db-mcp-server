# Cycle 02 — Real Transactions Replacing Stubs

**Status:** ✅ Shipped · **Artifacts:** PR #85, commit `e2598b5`

## Research Findings
- `DatabaseUseCase.ExecuteTransaction` was a stub: `begin` opened a transaction then committed it immediately ("we just commit right away"); `execute`, `commit`, `rollback` returned hardcoded success strings without touching any database. Discarded parameters (`txID`, `statement`, `params`) were the tell.
- A legacy `*sql.Tx` registry existed in `pkg/dbtools` but the live delivery path never used it.

## Shipped
- Mutex-guarded transaction registry inside `DatabaseUseCase` keyed by generated `transactionId`.
- All four actions real: `begin` opens + registers; `execute` runs statements inside the stored transaction (read-only SQL guard applied as defense-in-depth); `commit`/`rollback` apply to the real transaction and retire the ID.
- Unknown or replayed transaction IDs fail with clear errors instead of faking success.

## Verification
- Registry unit tests (store/route/retire semantics, read-only write-block inside transactions).
- End-to-end test against in-memory SQLite proving rollback discards staged writes and commit persists them (`internal/usecase/tx_e2e_test.go`).

## Fed Forward
Pattern learned: grep discarded `_` params to find other stubs.

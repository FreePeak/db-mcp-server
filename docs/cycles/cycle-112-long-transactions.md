# Cycle 112 — Idle-in-Transaction Audit (performance action=long_transactions)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A transaction left open (app bug, forgotten psql window) holds locks,
  blocks vacuum from reclaiming dead tuples on Postgres, and starves
  replication. `list_sessions` filters `state <> 'idle'` so idle-in-
  transaction rows appear, but nothing surfaces transaction age;
  `ListLongQueries` covers running statements only. Confirmed gap.

## Shipped

- `internal/usecase/long_transactions.go`:
  `ListLongTransactions(ctx, dbID, minAgeSecs)` — Postgres targets
  `pg_stat_activity WHERE state = 'idle in transaction'` with
  `xact_age`; MySQL reads `information_schema.innodb_trx` with age in
  seconds. Default threshold 60s; explicit clean state; SQLite errors
  "not available". Multi-line last-query collapsed inline (`oneLine`).
- Performance tool: new action `long_transactions` (both per-db and
  unified constructors) with optional `min_age_secs` param, served via
  the existing `sessionObservabilityUseCase` capability interface.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestLongTransactionsCatalog`: pg hits `idle in transaction` +
    `xact_start`; mysql hits innodb_trx/trx_started; sqlite none.
  - `TestListLongTransactions_Unsupported`: explicit error.
- Test-design iteration: initial pg filter listed ALL open
  transactions; tightened to idle-in-transaction specifically since
  running-long queries are already list_sessions/ListLongQueries
  territory.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for performance action=long_transactions.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 62 — Lock-Wait View + README Catch-Up

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Follow-on from cycle 61: listing sessions is half the debugging loop;
  when a query hangs, the agent needs to know WHO blocks WHOM before
  choosing a cancel target.
- Postgres: `pg_blocking_pids(pid)` via lateral unnest joined back to
  pg_stat_activity gives waiter/blocker pairs with both queries. MySQL:
  `sys.innodb_lock_waits` (requires the sys schema, standard on 5.7+/8.0).

## Shipped

- `internal/usecase/session_ops.go`: `blockingWaitsQuery(dbType)` +
  `ListBlockingWaits(ctx, dbID)` (capped at 100 rows, queries truncated to
  120 chars; explicit not-supported error elsewhere).
- Performance tool action `lock_waits` routed through the existing
  capability interface.
- README catch-up for cycles 59-62: query tool `format` param,
  generate_schema row now describes the real implementation, performance
  actions list list_sessions / lock_waits / cancel_query.

## Verification

- TDD RED first (undefined symbols → build fail), then GREEN:
  - `TestBlockingWaitsQuery`: pg_blocking_pids present, innodb_lock_waits
    present, unsupported engines return "".
  - `TestListBlockingWaits_SQLiteUnsupported`: explicit not-supported.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Oracle session view (v$session) behind the cloud harness.
- Cross-DB schema compare.
- Optional --export-dir for server-side file dumps with path sandboxing.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 127 — Deadlock-Counter Audit (performance action=deadlock_counts)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- lock_waits shows who blocks whom right now, but cumulative deadlock
  events are the durable evidence that a concurrency bug fires in
  production — the engine resolves each deadlock by killing a victim
  query and moving on, leaving only a counter behind.
  pg_stat_database.deadlocks / Innodb_deadlocks were unread from the
  tool surface. Confirmed absent.

## Shipped

- `internal/usecase/deadlocks.go`: `CheckDeadlocks(ctx, dbID)` —
  Postgres reads pg_stat_database.deadlocks for the current database;
  MySQL reads Innodb_deadlocks from performance_schema.global_status
  (reuses `toInt` for the string status value). Zero renders "No
  deadlocks recorded since stats reset."; <10 points at engine logs
  for victim queries; ≥10 adds a recurring-conflict warning. SQLite and
  other engines error "not available".
- Performance tool: new action `deadlock_counts` (both per-db and
  unified constructors) served via capability interface
  `deadlockUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDeadlockCatalog`: pg hits pg_stat_database + deadlocks; mysql
    hits Innodb_deadlocks; sqlite none.
  - `TestCheckDeadlocks_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=deadlock_counts.
- Post-merge: verify npm v1.12.0 + docker tags published.

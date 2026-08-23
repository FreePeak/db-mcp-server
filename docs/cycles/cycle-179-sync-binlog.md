# Cycle 179 — sync_binlog Durability Audit (performance action=sync_binlog)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `sync_binlog=0` disables fsync of the binary log: unflushed
  transactions live in the OS page cache, and a crash can lose
  commits replicas already received — silently diverging failover
  targets. Pairs with innodb_doublewrite (torn pages) as the MySQL
  crash-safety pair.
- Values >0 are a deliberate bounded tradeoff (group commit every N
  binlogs) and are NOT flagged; only full disablement escalates.
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/sync_binlog.go`:
  - `syncBinlogProbe` — @@GLOBAL probe; mysql/mariadb only.
  - `syncBinlogVerdict` — pure classifier: =0 → WARNING naming loss
    mode + SET GLOBAL fix; <0 → unreadable note; ≥1 → "" (audit adds
    explicit healthy line).
  - `AuditSyncBinlog` — runs the probe; unparseable values render as
    unreadable (Sscanf sentinel −1), never guessed at; unsupported
    engines get an explicit error.
- Performance tool: new action `sync_binlog` (both per-db and unified
  constructors) served via capability interface `syncBinlogUseCase`.

## Verification

- TDD RED first (build fail), GREEN on first run of implementation —
  no fixes needed this cycle.
- Tests: probe shape + engine gating; 1 and 1000 quiet (=1 durable,
  >1 deliberate group-commit); 0 escalated with loss mode + fix;
  -1 unreadable; explicit non-MySQL unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=sync_binlog.
- Post-merge: verify npm v1.12.0 + docker tags published.

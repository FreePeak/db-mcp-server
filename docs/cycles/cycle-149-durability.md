# Cycle 149 — Crash-Durability Audit (performance action=durability)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- innodb_flush_log_at_trx_commit=1 flushes the redo log at every
  commit — the ACID default. Values 2 and 0 defer that flush, so an OS
  crash or MySQL crash can lose up to ~1 second of COMMITTED
  transactions. Often set by benchmark-chasing; rarely revisited.
  MyISAM already covered by cycle 118's audit; this setting was not.
  Confirmed absent.

## Shipped

- `internal/usecase/durability.go`:
  - `flushLogQuery` — @@GLOBAL.innodb_flush_log_at_trx_commit;
    mysql/mariadb only.
  - `flushLogVerdict` — pure classifier: mode 1 healthy; mode 2 →
    WARNING (OS-crash loss, names SET GLOBAL fix); mode 0 → stronger
    WARNING (engine-crash loss); nonstandard values get a verify note;
    unparseable renders via -1 → verify path.
- Performance tool: new action `durability` (both per-db and unified
  constructors) served via capability interface `durabilityUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDurabilityProbe`: probe shape + engine gating.
  - `TestFlushLogVerdict`: 1 healthy, 2/0 escalated naming committed
    transaction risk and SET GLOBAL fix.
  - `TestAuditDurability_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=durability.
- Post-merge: verify npm v1.12.0 + docker tags published.

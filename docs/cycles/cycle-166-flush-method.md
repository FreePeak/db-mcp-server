# Cycle 166 — innodb_flush_method Audit (performance action=flush_method)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- On Linux the default `innodb_flush_method` (empty = fsync)
  double-buffers every write: data lands in both the InnoDB buffer
  pool and the OS page cache, wasting RAM and adding checkpoint
  stalls. `O_DIRECT` (and its NO_FSYNC variant) bypasses the page
  cache. A classic forgotten tuning — servers migrated from defaults
  keep paying the double-buffer tax forever. Windows is unaffected,
  so the warning phrases itself as Linux-specific.
- Adjacent coverage already shipped: crash_safety (PG
  fsync/full_page_writes), durability (MySQL flush-at-commit),
  doublewrite (torn pages). This closes the write-path quartet.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/flush_method.go`:
  - `flushMethodQuery` — @@GLOBAL.innodb_flush_method;
    mysql/mariadb only.
  - `flushMethodVerdict` — pure classifier: O_DIRECT /
    O_DIRECT_NO_FSYNC → "" (audit adds the explicit clean line);
    empty or any other method → WARNING naming the current method,
    the double-buffer consequence, and the my.cnf + restart fix.
  - `AuditFlushMethod` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `flush_method` (both per-db and
  unified constructors) served via capability interface
  `flushMethodUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestFlushMethodProbe`: probe shape + engine gating.
  - `TestFlushMethodVerdict`: both O_DIRECT variants render empty;
    empty/default escalated with fsync→O_DIRECT wording +
    config/restart fix.
  - `TestAuditFlushMethod_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=flush_method.
- Post-merge: verify npm v1.12.0 + docker tags published.

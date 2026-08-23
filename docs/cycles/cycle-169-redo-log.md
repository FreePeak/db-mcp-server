# Cycle 169 — Redo-Log Sizing Audit (performance action=redo_log)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- An undersized InnoDB redo log forces aggressive checkpointing:
  every write burst re-dirties pages the flusher just wrote,
  multiplying physical I/O. The historical default was ~48MB×2 files
  (5.x era); servers migrated forward keep paying it. MySQL 8.0.30+
  replaced `innodb_log_file_size` with
  `innodb_redo_log_capacity`, which is SET GLOBAL-able live; older
  versions need my.cnf + restart.
- Mirrors cycle 168's PG wal_compression finding on the MySQL side;
  completes the write-path/redo family alongside flush_method,
  io_capacity, and doublewrite. Confirmed absent from the tool
  surface.

## Shipped

- `internal/usecase/redo_log.go`:
  - `redoLogQueries` — preference ladder: modern capacity first
    (@@GLOBAL.innodb_redo_log_capacity), then legacy file-size math
    (innodb_log_file_size × innodb_log_files_in_group); mysql/mariadb
    only.
  - `redoLogVerdict` — pure classifier: <512MiB total → WARNING
    naming aggressive checkpointing/write amplification with both the
    modern live fix and the legacy restart path; ≥512MiB → "" (audit
    adds the explicit clean line); zero/unreadable → verify note.
  - `AuditRedoLog` — walks the probe ladder (older servers without
    the capacity variable fall through), parses defensively, renders
    verdict or healthy line via humanBytes; unsupported engines get
    an explicit error.
- Performance tool: new action `redo_log` (both per-db and unified
  constructors) served via capability interface `redoLogUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestRedoLogProbe`: ladder shape + engine gating.
  - `TestRedoLogVerdict`: 2GiB renders empty; 48MB escalated naming
    checkpointing and innodb_redo_log_capacity SET GLOBAL; zero
    flagged.
  - `TestAuditRedoLog_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=redo_log.
- Post-merge: verify npm v1.12.0 + docker tags published.

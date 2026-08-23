# Cycle 188 — log_lock_waits Audit (performance action=log_lock_waits)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With the default `off`, any lock wait longer than `deadlock_timeout`
  is invisible in the logs. The `lock_waits` action shows only what
  blocks *right now* — once the incident passes there is no durable
  evidence of who blocked whom or how long.
- Turning it on costs nothing until a wait actually happens; waits
  are logged against `deadlock_timeout` (default 1s).
- Fix is reloadable: ALTER SYSTEM + pg_reload_conf(), named in the
  warning.
- Candidate sweep: wal_compression / max_slot_wal_keep_size /
  log_min_duration_statement / track_io_timing / password_encryption /
  synchronous_commit already shipped; idle-replication-slots covered
  by stale_slots; full_page_writes covered by fsync-family audits.

## Shipped

- `internal/usecase/log_lock_waits.go`:
  - `logLockWaitsProbe` — reads `current_setting('log_lock_waits')`;
    postgres only.
  - `logLockWaitsVerdict` — pure classifier: on → "" (audit adds
    explicit healthy line); off → WARNING naming the never-logged
    failure mode and the reloadable fix; blank/other → unreadable
    note.
  - `AuditLogLockWaits` — runs the probe; empty values fall to the
    unreadable note; unsupported engines get an explicit error.
- Performance tool: new action `log_lock_waits` (both per-db and
  unified constructors) served via capability interface
  `logLockWaitsUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe shape + engine gating; on quiet; off escalated with
  fix + deadlock_timeout named; blank renders unreadable; explicit
  non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=log_lock_waits.
- Post-merge: verify npm v1.12.0 + docker tags published.

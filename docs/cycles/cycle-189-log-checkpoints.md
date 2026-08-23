# Cycle 189 — log_checkpoints Audit (performance action=log_checkpoints)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With the flag off (the default on PG ≤14), checkpoint
  start/finish timing, sync duration, and buffers-written counts
  never reach the logs — exactly the evidence needed to tune
  `max_wal_size` / `checkpoint_timeout` after an I/O stall.
- PG15+ defaults it on; older engines keep flying blind. The warning
  names that caveat so operators on modern versions don't churn.
- Fix is reloadable: ALTER SYSTEM + pg_reload_conf().
- Candidate sweep: innodb_flush_log_at_trx_commit / slow_query_log /
  long_query_time already shipped; performance_schema heavily used;
  autovacuum_naptime / maintenance_work_mem left as future
  candidates.

## Shipped

- `internal/usecase/log_checkpoints.go`:
  - `logCheckpointsProbe` — reads `current_setting('log_checkpoints')`;
    postgres only.
  - `logCheckpointsVerdict` — pure classifier: on → "" (audit adds
    explicit healthy line); off → WARNING naming invisible checkpoint
    evidence + the reloadable fix + PG15+ default note; blank/other →
    unreadable note.
  - `AuditLogCheckpoints` — runs the probe; empty values fall to the
    unreadable note; unsupported engines get an explicit error.
- Performance tool: new action `log_checkpoints` (both per-db and
  unified constructors) served via capability interface
  `logCheckpointsUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe shape + engine gating; on quiet; off escalated with
  fix + max_wal_size named; blank renders unreadable; explicit non-PG
  unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=log_checkpoints.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 176 — innodb_log_buffer_size Audit (performance action=log_buffer_size)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Transactions buffer their WAL in the InnoDB log buffer and flush
  at commit. When a transaction's writes exceed the buffer (16MB
  default), InnoDB must flush mid-transaction — visible as
  `Innodb_log_waits`, the engine's own evidence the buffer is too
  small for the workload.
- Evidence-driven escalation: nonzero waits → WARNING naming the
  overflow count and the fix (SET GLOBAL innodb_log_buffer_size='64M',
  dynamic on MySQL 8.0, restart before that; persist via my.cnf /
  SET PERSIST). Below-default size with zero waits gets a soft note;
  adequate size with no waits renders "" (audit adds explicit clean
  line).
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/log_buffer.go`:
  - `logBufferProbe` — setting + wait-counter SELECT; mysql/mariadb
    only.
  - `logBufferVerdict` — pure classifier: waits>0 → WARNING;
    <16MB clean → below-default note; else "" (healthy);
    ≤0/negative → unreadable note (parseSettingInt sentinel).
  - `AuditLogBuffer` — runs the probe, renders verdict or explicit
    healthy line; unsupported engines get an explicit error.
  - Reused existing `parseSettingInt` + `humanBytes` helpers instead
    of adding duplicate parsers.
- Performance tool: new action `log_buffer_size` (both per-db and
  unified constructors) served via capability interface
  `logBufferUseCase`.

## Verification

- TDD RED first (build fail), then GREEN after one test-side fix
  (assertion grepped "below the default" vs actual "below the 16M
  default") — implementation unchanged.
  - `TestLogBufferProbe`: probe shape (setting + evidence counter)
    + engine gating.
  - `TestLogBufferSizeVerdict`: healthy quiet; waits=42 escalated
    naming Innodb_log_waits and the live fix; shrunken-clean noted;
    unreadable flagged.
  - `TestAuditLogBuffer_Unsupported`: explicit non-MySQL error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=log_buffer_size.
- Post-merge: verify npm v1.12.0 + docker tags published.

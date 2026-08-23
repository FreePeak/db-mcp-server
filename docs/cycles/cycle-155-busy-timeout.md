# Cycle 155 — SQLite busy_timeout Audit (performance action=busy_timeout)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- SQLite's busy_timeout defaults to 0 ms: any lock contention fails
  immediately with SQLITE_BUSY instead of waiting for the lock to
  clear. It is the companion fix to cycle 153's WAL audit (WAL still
  allows exactly one writer): with a retry window set, concurrent
  access degrades to brief waits rather than errors. Confirmed absent
  from the tool surface.

## Shipped

- `internal/usecase/busy_timeout.go`:
  - `busyTimeoutQuery` — PRAGMA busy_timeout; sqlite/sqlite3 only.
  - `busyTimeoutVerdict` — pure classifier: 0 → WARNING naming the
    immediate-failure behavior and the per-connection PRAGMA fix;
    >0 renders "" (audit adds the explicit clean line).
  - `AuditBusyTimeout` — runs the pragma against the live database,
    parses the counter defensively, renders verdict or healthy line;
    unsupported engines get an explicit error.
- Performance tool: new action `busy_timeout` (both per-db and unified
  constructors) served via capability interface `busyTimeoutUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestBusyTimeoutProbe`: probe shape + engine gating.
  - `TestBusyTimeoutVerdict`: 5000ms renders empty; 0 escalated with
    "busy" + PRAGMA fix.
  - `TestAuditBusyTimeout_EndToEnd`: audit runs against a real SQLite
    database via the standard test harness.
  - `TestAuditBusyTimeout_Unsupported`: explicit error for non-SQLite.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=busy_timeout.
- Post-merge: verify npm v1.12.0 + docker tags published.

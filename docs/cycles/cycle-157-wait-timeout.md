# Cycle 157 — wait_timeout Audit (performance action=wait_timeout)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- MySQL's wait_timeout closes connections idle longer than the
  setting; both extremes hurt operationally:
  - Too high (the 28800s = 8h default): idle connections from crashed
    clients hold pool slots until "too many connections".
  - Too low (<30s): pooled connections are dropped server-side
    mid-idle, surfacing as "server has gone away" errors.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/wait_timeout.go`:
  - `waitTimeoutQuery` — @@GLOBAL.wait_timeout; mysql/mariadb only.
  - `waitTimeoutVerdict` — two-sided classifier over constants
    `waitTimeoutFloorSecs` (30s) / `waitTimeoutCeilSecs` (8h): ≤0 →
    unreadable note; low → WARNING naming 'server has gone away' with
    SET GLOBAL fix + client-pool match; high → WARNING naming slot
    exhaustion with SET GLOBAL fix; in-band renders "" (audit adds the
    explicit clean line). `humanHours` renders durations readably.
  - `AuditWaitTimeout` — runs the probe, parses defensively, renders
    verdict or healthy band line; unsupported engines get an explicit
    error.
- Performance tool: new action `wait_timeout` (both per-db and unified
  constructors) served via capability interface `waitTimeoutUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestWaitTimeoutProbe`: probe shape + engine gating.
  - `TestWaitTimeoutVerdict`: 600s renders empty; 7-day timeout
    escalated with idle/SET GLOBAL; 5s escalated with 'gone away'; 0
    flagged unreadable.
  - `TestAuditWaitTimeout_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=wait_timeout.
- Post-merge: verify npm v1.12.0 + docker tags published.

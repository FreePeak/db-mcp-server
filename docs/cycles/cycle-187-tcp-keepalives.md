# Cycle 187 — tcp_keepalives_idle Audit (performance action=tcp_keepalives_idle)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `tcp_keepalives_idle` sets how long a connection must sit idle
  before the OS sends TCP keepalive probes. The default (0 = OS
  default, often 2 hours on Linux) lets dead clients — closed
  laptops, dropped NAT sessions — hold connection slots for hours.
- `idle_sessions` shows the symptom; this names the config-level
  fix so slots free themselves in minutes: ALTER SYSTEM +
  pg_reload_conf() to 300s.
- Escalation ladder: >0 and ≤600s → quiet; 0 → WARNING naming the
  OS-default risk; large values → WARNING with the duration
  rendered; negative/non-numeric → "unreadable" note rather than a
  guess.
- Confirmed absent from the tool surface; wal_compression and
  idle-replication-slots candidates were already covered by
  existing audits.

## Shipped

- `internal/usecase/tcp_keepalives.go`:
  - `tcpKeepalivesProbe` — reads `current_setting('tcp_keepalives_idle')`
    in seconds; postgres only.
  - `tcpKeepaliveVerdict` — pure classifier with the ladder above;
    healthy floors render "" (audit adds explicit healthy line).
  - `AuditTCPKeepalives` — runs the probe; non-numeric values fall
    to the unreadable note; unsupported engines get an explicit
    error.
- Performance tool: new action `tcp_keepalives_idle` (both per-db
  and unified constructors) served via capability interface
  `tcpKeepalivesUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe shape + engine gating; 60s and 600s quiet; 0
  escalated with dead-client mode + named fix; 7200s renders its
  duration; −1 renders unreadable; explicit non-PG unsupported
  error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=tcp_keepalives_idle.
- Post-merge: verify npm v1.12.0 + docker tags published.

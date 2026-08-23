# Cycle 181 — wal_sender_timeout Audit (performance action=wal_sender_timeout)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `wal_sender_timeout` is the walsender heartbeat that reaps dead
  standbys. `0` disables it: a crashed replica's walsender never
  exits, its replication slot keeps pinning WAL, and pg_wal grows
  until disk fills. Very low values kill healthy-but-slow standbys on
  flaky links, triggering reconnect storms.
- Escalation ladder: 0 → disabled warning with ALTER SYSTEM +
  reload fix; <10s → too-aggressive warning; sane → "" (audit adds
  explicit healthy line).
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/wal_sender_timeout.go`:
  - `walSenderTimeoutProbe` — current_setting probe; postgres only.
  - `parseTimeoutSecs` — interval parser ("60s", "1min", "500ms",
    bare number = ms); unparseable → unreadable.
  - `walSenderTimeoutVerdict` — pure classifier per the ladder.
  - `AuditWalSenderTimeout` — runs the probe; unsupported engines
    get an explicit error.
- Performance tool: new action `wal_sender_timeout` (both per-db and
  unified constructors) served via capability interface
  `walSenderTimeoutUseCase`.

## Verification

- TDD RED first (build fail), then two GREEN fixes — both were test
  assertions using different wording than the verdict text
  ("never reaped" vs the shipped "detection is disabled"; "flaky
  network" vs "flaky links"). Implementation logic was correct; the
  assertions were aligned to the actual verdict wording.
- Tests: probe shape + engine gating; "60s" quiet; "0" escalated
  with slot-pinning loss mode + fix; "2s" aggressive escalation;
  ""/"soon" unreadable; explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=wal_sender_timeout.
- Post-merge: verify npm v1.12.0 + docker tags published.

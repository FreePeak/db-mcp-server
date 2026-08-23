# Cycle 184 — checkpoint_timeout Audit (performance action=checkpoint_timeout)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `checkpoint_timeout` sets how often PostgreSQL force-checkpoints.
  Too short → checkpoint storms: every dirty page earns full-page
  writes into WAL each cycle (inflated WAL volume + periodic I/O
  spikes that read as random slowdowns). Too long → crash recovery
  must replay up to that much WAL before the server accepts
  connections.
- The 5-minute default is conservative; 15–30 minutes suits most
  write-heavy workloads. Ladder: <300s storm warning; >3600s
  recovery warning; between → quiet (audit adds explicit healthy
  line). Reloadable via pg_reload_conf(), so the fix names that.
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/checkpoint_timeout.go`:
  - `checkpointTimeoutProbe` — reads the setting as epoch seconds
    via `EXTRACT(EPOCH FROM current_setting(...)::interval)`; postgres only.
  - `checkpointTimeoutVerdict` — pure classifier: unreadable /
    storm (<300s) / recovery (>3600s) / quiet, each warning naming
    the ALTER SYSTEM + pg_reload_conf() fix.
  - `AuditCheckpointTimeout` — runs the probe; unparseable values
    render as unreadable (sentinel −1), never guessed at;
    unsupported engines get an explicit error.
- Performance tool: new action `checkpoint_timeout` (both per-db and
  unified constructors) served via capability interface
  `checkpointTimeoutUseCase`.

## Verification

- TDD RED first (build fail), then one GREEN fix: the storm message
  said "full-page write" where the test asserts plural "full-page
  writes" — wording aligned with the assertion.
- Tests: probe uses EXTRACT epoch seconds + engine gating; 900s
  quiet; 60s escalated with full-page-writes mode + named fix;
  7200s escalated with crash-recovery mode; −1 unreadable; explicit
  non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=checkpoint_timeout.
- Post-merge: verify npm v1.12.0 + docker tags published.

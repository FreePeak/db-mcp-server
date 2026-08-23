# Cycle 156 — track_io_timing Audit (performance action=track_io_timing)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With `track_io_timing` off (the PostgreSQL default), EXPLAIN ANALYZE
  reports no I/O timings and pg_stat views lack block read/write time
  — silently degrading exactly the tooling agents rely on to tell
  CPU-bound from disk-bound queries. The measurement overhead is small
  on modern kernels. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/io_timing.go`:
  - `trackIoTimingQuery` — current_setting('track_io_timing');
    postgres/postgresql only.
  - `trackIoTimingVerdict` — pure classifier: off → WARNING naming the
    hidden I/O timings with the ALTER SYSTEM fix; on → "" (audit adds
    the explicit clean line); empty/unreadable → verify hint.
  - `AuditTrackIoTiming` — runs the probe against the live database;
    unsupported engines get an explicit error.
- Performance tool: new action `track_io_timing` (both per-db and
  unified constructors) served via capability interface
  `ioTimingUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestTrackIoTimingProbe`: probe shape + engine gating.
  - `TestTrackIoTimingVerdict`: on renders empty; off escalated with
    "I/O"/"EXPLAIN" + ALTER SYSTEM fix; empty flagged unreadable.
  - `TestAuditTrackIoTiming_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=track_io_timing.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 195 — MySQL Slow-Query-Log Audit (performance action=slow_query_log)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With `slow_query_log=OFF` or the default `long_query_time=10s`,
  slow queries never land anywhere an agent can inspect —
  `engine_slow_queries` (performance schema / sys digests) silently
  sees nothing, and "the database feels slow" becomes undiagnosable.
- Both fixes are runtime `SET GLOBAL` — no restart required, so the
  warning names the exact statement.
- Threshold ladder: OFF → disabled warning; ≥5 s (covers the 10 s
  default) → threshold warning pointing at the 0.1–2 s incident band;
  otherwise quiet. Healthy configs render explicitly.

## Shipped

- `internal/usecase/slow_query_log.go`:
  - `slowQueryLogProbe` — reads `@@GLOBAL.slow_query_log` +
    `@@GLOBAL.long_query_time`; mysql/mariadb only.
  - `slowQueryLogVerdict` — pure classifier per the ladder above.
  - `AuditSlowQueryLog` — runs the probe; unparseable
    `long_query_time` is an explicit error rather than a silent 0.
- Performance tool: new action `slow_query_log` (per-db + unified)
  via capability interface `slowQueryLogUseCase`.

## Verification

- TDD RED first, GREEN after implementation. Two GREEN-phase catches:
  undefined `parseBoolSetting` (replaced with the existing
  `truthySetting` helper from crash_safety.go), and the recurring
  case-sensitivity trap third time running — test asserted lowercase
  `"disabled"` while the message rendered `"DISABLED"`; message
  normalized again instead of loosening the assertion.
- golangci-lint flagged unchecked `strconv.ParseFloat` error → now an
  explicit unparseable-threshold error path (strictly better).
- Tests: probe shape + engine gating; verdict ladder table (healthy
  quiet, disabled escalated with SET GLOBAL fix, default 10 s
  escalated); explicit non-MySQL unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=slow_query_log.
- Post-merge: verify npm v1.12.0 + docker tags published.

## Loop Status

- The config-audit sweep family is now: autovacuum_naptime,
  effective_io_concurrency, replication_slots, slow_query_log,
  crash_safety, doublewrite, binlogs, wait_timeout, packet_size,
  flush_neighbors, auto_increment, table_cache, long_transactions,
  sessions/blocking/cancel. Remaining niche candidates (wal_buffers,
  hot_standby_feedback) are lower value; next cycles shift to
  hardening passes or closing documented-but-missing surfaces.

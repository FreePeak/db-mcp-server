# Cycle 171 — Autovacuum Throttle Audit (performance action=autovacuum_throttle)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The legacy autovacuum cost budget
  (`autovacuum_vacuum_cost_delay=2ms`, limit inherited from
  `vacuum_cost_limit=200`) was calibrated for spinning disks. On
  modern storage with busy write-heavy tables, throttled autovacuum
  loses the race: dead tuples accumulate, bloat grows, indexes fatten
  until every query pays. Raising the limit (or zeroing the delay)
  lets vacuum keep pace; per-table autovacuum-off is a separate audit
  (action=vacuum_disabled), as is wraparound pressure.
- Both settings are sighup-context: ALTER SYSTEM + pg_reload_conf(),
  no restart. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/av_throttle.go`:
  - `avThrottleQuery` — reads both cost settings via
    current_setting(); postgres only.
  - `parseGUCms` — GUC time-value parser ("2ms", "0", "100",
    "500us", "1s", "1min") → milliseconds.
  - `avThrottleVerdict` — pure classifier: unthrottled (delay=0) or
    raised budget (limit≥500) → ""; legacy spinning-disk budget →
    WARNING naming bloat accumulation and the ALTER SYSTEM +
    pg_reload_conf fix with a dead-tuple watch note; empty/unreadable
    → verify note.
  - `AuditAVThrottle` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `autovacuum_throttle` (both per-db and
  unified constructors) served via capability interface
  `avThrottleUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestAVThrottleProbe`: both settings present + engine gating.
  - `TestAVThrottleVerdict`: unthrottled/raised render empty; "2ms"/
    "-1" escalated naming the spinning-disk budget and fix path;
    empty flagged unreadable.
  - `TestAuditAVThrottle_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=autovacuum_throttle.
- Post-merge: verify npm v1.12.0 + docker tags published.

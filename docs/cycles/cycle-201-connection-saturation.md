# Cycle 201 — Connection saturation audit (`connection_saturation`)

**Theme:** health_audit gains a max_connections headroom check — the
"too many connections" outage is visible before it happens.

## Objective

An engine that hits `max_connections` rejects every new session with a
cryptic error. The health tool already surfaces pool state for *this
server's* connections; nothing reports global connection pressure.
Add an auditor that reads the live counter vs the configured ceiling and
escalates on a ladder.

## Research findings

- MySQL: `SHOW GLOBAL STATUS LIKE 'Threads_connected'` +
  `SHOW VARIABLES LIKE 'max_connections'`; also `Max_used_connections`
  (historical peak) is available but two probes keep the shape uniform —
  the live ratio is what matters for "will it fail right now".
- PostgreSQL: one probe — `SELECT count(*) FROM pg_stat_activity` vs
  `current_setting('max_connections')`. Note: superuser_reserved_slots
  (default 3) means non-superuser exhaustion lands slightly before 100%,
  so thresholds are conservative.
- SQLite: single-embedded-writer model, no connection ceiling →
  unsupported (probe returns `""`, auditor omitted from report).

## Shipped

- `internal/usecase/connection_saturation.go`
  - `connectionSaturationQuery(dbType)` — engine catalog SELECT returning
    `(current, maximum)` or `""` when unsupported.
  - `connectionSaturationVerdict(cur, max)` — escalation ladder:
    - ≥ 100%: **CRITICAL** — new sessions rejected now.
    - ≥ 90%: **WARNING** — minutes from rejection under load burst;
      raise ceiling or find the leak.
    - < 90%: clean line with current/max and headroom.
  - `CheckConnectionSaturation(ctx, dbID)` — runs the probe, renders the
    verdict; scan errors render as unreadable rather than failing.
- Registered in `configAuditors` as `"connection_saturation"` (58 entries).

## Verification evidence

- `TestConnectionSaturationQuery`: postgres/mysql return their probes,
  sqlite returns `""`.
- `TestConnectionSaturationVerdict`: full matrix — 100%+ CRITICAL,
  90–99% WARNING, below-90% clean, zero-max treated as unreadable.
- `go build ./... && go vet ./... && go test ./... -count=1` — all pass.
- `golangci-lint run` — clean.

## Artifacts

- Commit: `feat(health): connection saturation audit in health_audit registry`

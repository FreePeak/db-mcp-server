# Cycle 192 — autovacuum_naptime Audit (performance action=autovacuum_naptime)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `autovacuum_naptime` (default 60s) is how long autovacuum waits
  between passes. Raised values delay every table's cleanup cadence,
  so dead-tuple bloat accumulates between passes on busy tables.
- Because the setting applies **per-database**, clusters with many
  databases multiply the effective per-table delay — the warning
  says so explicitly.
- Ladder: ≤300s → quiet (audit adds explicit healthy line);
  >300s → WARNING with the reloadable fix
  (`ALTER SYSTEM SET autovacuum_naptime = '60s'` + pg_reload_conf).
- Distinct from existing audits: `autovacuum_off` covers per-table
  opt-outs, `av_throttle` covers the cost budget; this one covers
  pass cadence.
- New helper `parseSecondsSetting` handles bare numbers and GUC
  suffixed forms ("1min", "2 min", "90s") via time.ParseDuration.

## Shipped

- `internal/usecase/autovacuum_naptime.go`:
  - `autovacuumNaptimeProbe` — reads
    `current_setting('autovacuum_naptime')`; postgres only.
  - `parseSecondsSetting` — seconds/interval parser ("min"→"m"
    normalization for Go's ParseDuration).
  - `autovacuumNaptimeVerdict` — pure classifier: ≤300s → "";
    >300s → escalated naming bloat accumulation, per-database
    multiplication, and the fix; ≤0 → unreadable note.
  - `AuditAutovacuumNaptime` — runs the probe; unsupported engines
    get an explicit error.
- Performance tool: new action `autovacuum_naptime` (both per-db
  and unified constructors) served via capability interface
  `autovacuumNaptimeUseCase`.

## Verification

- TDD RED first, GREEN after implementation. One implementation
  fix during GREEN: Go's ParseDuration rejects "min" spelling, so
  the parser normalizes "min"→"m" before parsing (test caught it).
- Tests: probe shape + engine gating; default-60s and boundary-300s
  quiet; 600s escalation naming bloat/per-database/fix; zero
  unreadable; explicit non-PG unsupported error; parser table test.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=autovacuum_naptime.
- Post-merge: verify npm v1.12.0 + docker tags published.

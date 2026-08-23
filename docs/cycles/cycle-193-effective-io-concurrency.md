# Cycle 193 — effective_io_concurrency Audit (performance action=effective_io_concurrency)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL's `effective_io_concurrency` default of `1` is
  calibrated for spinning disks where seeking dominates. On SSD/NVMe
  storage (nearly all modern deployments) bitmap-heap scans prefetch
  one page at a time and read large ranges serially — the device
  queue sits idle during big scans.
- Raising to ~200 lets the kernel keep the SSD queue full; the
  warning names the reloadable fix
  (`ALTER SYSTEM SET ... = 200` + pg_reload_conf).
- Value `0` disables prefetch entirely and renders its own note.
- Mirrors the earlier `innodb_flush_neighbors` finding on the MySQL
  side: same root cause (spinning-disk defaults), opposite knob.

## Shipped

- `internal/usecase/effective_io_concurrency.go`:
  - `effectiveIOConcurrencyProbe` — reads
    `current_setting('effective_io_concurrency')`; postgres only.
  - `effectiveIOConcurrencyVerdict` — pure classifier: ≥200 → "";
    1–199 → spinning-disk warning naming prefetch + fix;
    ≤0 → disabled/unreadable note.
  - `AuditEffectiveIOConcurrency` — runs the probe; unsupported
    engines get an explicit error.
- Performance tool: new action `effective_io_concurrency` (both
  per-db and unified constructors) served via capability interface
  `effectiveIOConcurrencyUseCase`.

## Verification

- TDD RED first, GREEN after implementation (no fixes needed this
  cycle).
- Tests: probe shape + engine gating; 200 quiet; default-1
  escalation naming spinning disk/prefetch/SSD/fix; zero disabled
  note; explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=effective_io_concurrency.
- Post-merge: verify npm v1.12.0 + docker tags published.

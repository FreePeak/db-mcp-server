# Cycle 167 — innodb_io_capacity Audit (performance action=io_capacity)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The default `innodb_io_capacity=200` was calibrated for a single
  spinning disk. Left untouched on SSD/NVMe-backed servers the
  background flusher stays lazy until dirty pages pile up, then write
  stalls arrive in bursts — and checkpoint pacing keys off the same
  wrong number. Both `innodb_io_capacity` and
  `innodb_io_capacity_max` are SET GLOBAL-able live; no restart.
- Adjacent coverage already shipped: flush_method (write path),
  doublewrite (torn pages), buffer_pool, checkpoint pressure. This
  closes the InnoDB flush-pacing gap. Confirmed absent from the tool
  surface.

## Shipped

- `internal/usecase/io_capacity.go`:
  - `ioCapacityQuery` — both settings in one round trip;
    mysql/mariadb only.
  - `ioCapacityVerdict` — pure classifier: ≤400 → WARNING naming the
    single-spinning-disk origin, the lazy-flush burst consequence,
    and the live SET GLOBAL fix; >400 → "" (audit adds the explicit
    clean line); zero/unreadable → verify note.
  - `AuditIOCapacity` — runs the probe, parses defensively,
    renders verdict or healthy line with actual values; unsupported
    engines get an explicit error.
- Performance tool: new action `io_capacity` (both per-db and unified
  constructors) served via capability interface `ioCapacityUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try for tests. One
  lint fix after wiring: revive rejected `cap_`/`max_` underscores —
  renamed to ioCap/ioCapMax before full verify.
  - `TestIOCapacityProbe`: reads both settings + engine gating.
  - `TestIOCapacityVerdict`: 8000/16000 renders empty; 200/2000
    escalated naming the value and the live fix; zero flagged.
  - `TestAuditIOCapacity_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=io_capacity.
- Post-merge: verify npm v1.12.0 + docker tags published.

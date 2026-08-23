# Cycle 130 — XID-Wraparound Risk Audit (performance action=wraparound_risk)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL assigns every write a 32-bit XID; when the oldest
  un-frozen XID approaches ~2 billion the engine stops accepting writes
  and forces an emergency single-user vacuum — the most catastrophic
  silent failure Postgres has, and entirely preventable if age is
  watched. Nothing on the tool surface read datfrozenxid. Confirmed
  absent.

## Shipped

- `internal/usecase/wraparound.go`:
  - `wraparoundQuery` — per-database `age(datfrozenxid)` from
    pg_database; Postgres-only, others error "not available".
  - `wraparoundVerdict(name, age)` — pure classifier with escalating
    thresholds: healthy / WARNING at ≥200M ("check why autovacuum is
    falling behind") / CRITICAL at ≥500M ("freeze aggressively NOW or
    writes stop at ~2.1B").
  - `CheckWraparoundRisk(ctx, dbID)` — renders all databases worst
    first; non-connectable template databases skipped by design;
    warning/critical results append a pointer at idle-in-transaction
    sessions (they hold back the freeze horizon).
- Performance tool: new action `wraparound_risk` (both per-db and
  unified constructors) served via capability interface
  `wraparoundUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestWraparoundCatalog`: hits pg_database + datfrozenxid; mysql ""
    (XIDs are PG-only).
  - `TestCheckWraparoundRisk_Unsupported`: explicit error.
  - `TestWraparoundVerdict`: young=healthy, ≥200M=WARNING, ≥500M=
    CRITICAL escalation proven.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=wraparound_risk.
- Post-merge: verify npm v1.12.0 + docker tags published.

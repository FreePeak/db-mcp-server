# Cycle 161 — wal_level Audit (performance action=wal_level)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL `wal_level=minimal` disables streaming replication,
  point-in-time recovery, and logical decoding entirely. It is a
  benchmark-era leftover that surfaces only when a replica fails to
  start or a PITR restore is needed — by then the WAL that would have
  enabled recovery is long gone. `replica` (the default) and
  `logical` are both healthy. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/wal_level.go`:
  - `walLevelQuery` — current_setting('wal_level');
    postgres/postgresql only.
  - `walLevelVerdict` — pure classifier: minimal → WARNING naming
    lost replication/PITR/logical decoding with the ALTER SYSTEM +
    restart fix; replica/logical → "" (audit adds the explicit clean
    line); empty/unreadable → verify note.
  - `AuditWALLevel` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `wal_level` (both per-db and unified
  constructors) served via capability interface `walLevelUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestWALLevelProbe`: probe shape + engine gating.
  - `TestWALLevelVerdict`: replica and logical render empty; minimal
    escalated with replication/PITR wording + ALTER SYSTEM fix; empty
    flagged unreadable.
  - `TestAuditWALLevel_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=wal_level.
- Post-merge: verify npm v1.12.0 + docker tags published.

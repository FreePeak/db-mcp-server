# Cycle 126 — Stale Replication-Slot Audit (performance action=stale_slots)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- replication_status shows attached replicas, but an INACTIVE slot is
  a different failure mode: its consumer died and PostgreSQL keeps
  retaining WAL on the slot's behalf until the disk fills, silently.
  Confirmed absent.

## Shipped

- `internal/usecase/wal_slots.go`: `ListStaleSlots(ctx, dbID)` — counts
  pg_replication_slots, then reads inactive slots with retained-WAL
  size via pg_wal_lsn_diff/pg_size_pretty. Explicit states: no slots,
  all active ("no WAL retention risk"), or per-slot warning lines
  naming the drop-or-fix decision before the disk fills. Only Postgres;
  other engines error "not available".
- Performance tool: new action `stale_slots` (both per-db and unified
  constructors) served via capability interface `staleSlotUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestStaleSlotCatalog`: hits pg_replication_slots + active +
    restart_lsn; mysql/sqlite empty (slots are PG-only).
  - `TestListStaleSlots_Unsupported`: explicit error.
- Self-caught during implementation: removed a leftover placeholder
  block before first compile.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=stale_slots.
- Post-merge: verify npm v1.12.0 + docker tags published.

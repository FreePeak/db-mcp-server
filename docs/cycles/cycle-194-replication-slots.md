# Cycle 194 — Replication-Slot Audit (performance action=replication_slots)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- An abandoned replication slot still forces the primary to retain
  every WAL segment the slot needs — disk fills with no error
  anywhere until the filesystem is full. Any retained WAL on an
  inactive slot is unbounded growth in waiting, so the warning names
  the drop command per slot (`pg_drop_replication_slot`).
- Slots also cap concurrent standbys: when usage reaches
  `max_replication_slots`, new replicas fail to attach with only a
  log line as evidence. Capacity-full is escalated explicitly.
- Retained bytes computed via `pg_wal_lsn_diff(pg_current_wal_lsn(),
  restart_lsn)`; NULL restart_lsn renders 0.
- Unreadable `max_replication_slots` skips the capacity check rather
  than failing the audit (slot findings still render).

## Shipped

- `internal/usecase/replication_slots.go`:
  - `replicationSlotsProbe` — per-slot SELECT with retained-WAL
    accounting; postgres only.
  - `replicationSlotVerdict` — pure classifier: inactive slots warn
    (with retention size + drop command); capacity full warns;
    all-active fleet quiet; empty fleet states health with usage.
  - `AuditReplicationSlots` — runs both probes and renders;
    unsupported engines get an explicit error.
- Performance tool: new action `replication_slots` (both per-db and
  unified constructors) served via capability interface
  `replicationSlotsUseCase`.

## Verification

- TDD RED first, GREEN after implementation. Two fixes during GREEN,
  both caught by tests: undefined helper references (replaced with a
  direct Sscanf parse of the setting) and case-sensitivity — the
  capacity message rendered "FULL" while the test asserted lowercase
  "full"; message normalized instead of loosening the assertion.
- Tests: verdict table (empty fleet healthy with usage, active-only
  quiet, inactive slot named with retention + WAL + drop fix, active
  slots never reported, exhausted capacity escalated); probe shape +
  engine gating; explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=replication_slots.
- Post-merge: verify npm v1.12.0 + docker tags published.

## Loop Status

- Remaining sweep candidates after this cycle: none high-value —
  wal_buffers / hot_standby_feedback are niche by comparison.
  Next cycles should shift to hardening passes (regression sweeps)
  or close out remaining README-documented-but-missing surfaces.

# Cycle 170 — max_slot_wal_keep_size Audit (performance action=slot_wal_cap)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With the default `max_slot_wal_keep_size=-1`, any replication slot
  — including one nobody remembers creating — retains WAL in pg_wal
  without bound until the disk fills. A cap converts that failure
  mode into bounded slot invalidation: a lagging slot gets dropped
  rather than taking the primary down. Stale-slot detection
  (action=stale_slots, wal_slots.go) watches existing slots
  observationally; this closes the missing-global-safety-net gap.
- PG13+ setting, reload-context (ALTER SYSTEM + pg_reload_conf(), no
  restart). Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/slot_wal_cap.go`:
  - `slotWalCapQuery` — current_setting('max_slot_wal_keep_size');
    postgres only.
  - `slotWalCapVerdict` — pure classifier: -1 → WARNING naming the
    forgotten-slot disk-fill failure and the ALTER SYSTEM +
    pg_reload_conf fix with invalidation semantics; positive value →
    "" (audit adds the explicit clean line with the actual cap);
    empty/unreadable → verify note.
  - `AuditSlotWalCap` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `slot_wal_cap` (both per-db and
  unified constructors) served via capability interface
  `slotWalCapUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestSlotWalCapProbe`: probe shape + engine gating.
  - `TestSlotWalCapVerdict`: capped "500GB" renders empty; "-1"
    escalated naming unbounded retention; empty flagged unreadable.
  - `TestAuditSlotWalCap_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=slot_wal_cap.
- Post-merge: verify npm v1.12.0 + docker tags published.

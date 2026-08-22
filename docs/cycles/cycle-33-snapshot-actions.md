# Cycle 33 — Agent-Facing Snapshot Actions

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 32's snapshots were usecase-internal — agents had no path to undo. cocaxcode exposes rollback as first-class tools; our transaction tool already owns mutation semantics, so snapshot actions belong there.

## Shipped
- Transaction tool gains `list_snapshots` (ID, kind, table, row count, timestamp per entry) and `rollback_snapshot` (requires `snapshot_id`) actions on per-db and unified variants.
- Capability-detected dispatch (`snapshotCapable`) keeps existing mocks/providers compatible; unknown provider fails with a clear message.
- Tool schema descriptions updated in both variants; contract-guard ready.

## Verification
- TDD RED-first at the delivery layer: rollback routes the exact snapshot ID and confirms in output; listing includes registered snapshots. Both failed before wiring, pass after.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- README tool-table line for the new transaction actions.
- Snapshot entries could carry the originating statement text for audit context.

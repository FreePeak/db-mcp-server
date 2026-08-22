# Cycle 36 — Agent-Facing Schema Drift Actions

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 35's drift engine was usecase-internal; agents verify migrations through tools, so the capability needed an MCP surface. Transaction tool already owns the mutation-lifecycle actions (snapshots, rollback) — schema lifecycle belongs beside them.

## Shipped
- `capture_schema_snapshot`: stores a baseline and prints it (table → column/type list) with the new baseline ID.
- `check_schema_drift` (requires `baseline_id`): renders "No schema drift detected" or a sorted change list.
- `list_schema_snapshots`: baseline enumeration with table counts and timestamps.
- Capability-detected dispatch (`schemaDriftCapable`); action descriptions updated in both tool variants.

## Verification
- TDD RED-first at delivery layer: capture returns new baseline ID, drift-check routes the exact baseline and renders changes, listing enumerates. All failed pre-wiring, pass after.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- README transaction-tool row should mention the schema-drift actions.
- Migration workflow recipe: capture → run DDL → drift check — document in examples/.

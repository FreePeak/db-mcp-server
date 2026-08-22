# Cycle 32 — Pre-Mutation Snapshots + Reverse-SQL Rollback

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- cocaxcode/database-mcp's standout safety feature: pre-mutation snapshots on every write with reverse-SQL undo. Nothing in the multi-engine Go server space offers it; our single ExecuteStatement seam made it a clean addition after the risk-analysis work.

## Shipped
- `ExecuteStatement` captures affected rows BEFORE any DELETE/UPDATE (top-level WHERE extraction with paren-depth scanning; literals stripped for table/kind detection), stores them in a bounded per-database ring (`snapshotCapacityPerDB=25`), and returns the snapshot ID in the result.
- `RollbackSnapshot(ctx, dbID, id)`: DELETE → re-INSERT captured rows column-exact; UPDATE → restore old values targeted by `id` column (clear error when the table lacks one). Quoted identifiers throughout.
- INSERT/reads create no snapshots (nothing reversible to capture).
- Best-effort by design: introspection failures never block execution.

## Verification
- TDD RED-first: delete→rollback round-trip restores exact rows, update→rollback reverses values, reads/INSERTs consume no slots, unknown-ID error semantics, ring-cap eviction under sustained churn.
- Bugs caught during GREEN: constructor refactor dropped the `riskWarnAt: "high"` default (everything warned); INSERT was initially snapshotted despite being irreversible — both caught by existing cycle tests, fixed.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Expose ListSnapshots/RollbackSnapshot as MCP tool actions (transaction tool `rollback_snapshot` action) so agents can self-serve undo.
- Durable snapshot spill for large row sets.

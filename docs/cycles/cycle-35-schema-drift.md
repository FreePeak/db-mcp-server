# Cycle 35 — Schema Snapshots & Drift Detection

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- boringSQL/dryrun's capability table includes schema snapshot + drift detection ("capture current schema state, compare and report changes") — migration verification is a recurring agent need: run DDL, then prove what actually moved.
- Our describe/schema catalog layer (cycles 7–16) already normalized per-engine introspection; diffing was the missing primitive.

## Shipped
- `CaptureSchemaSnapshot(ctx, dbID)`: normalized capture of every table's ordered columns (name+type, lowercased) via the existing GetDatabaseInfo/DescribeTable path; stored as bounded per-db baseline ring (20).
- `CheckSchemaDrift(ctx, dbID, baselineID)`: reports added/removed tables, added/removed columns, type changes (before → after), with `drifted` flag; deterministic sorted output.
- `ListSchemaSnapshots(dbID)` for baseline enumeration.

## Verification
- TDD RED-first: normalized capture (2 tables, users.email typed), full change-class matrix on evolved SQLite (column add + table add + table drop all reported), clean-comparison silence, unknown-baseline error, baseline enumeration.
- Bugs caught during GREEN: catalog key variance (`table_name` vs `name`, `column_name` alias), empty snapshot ID never assigned, global store leaking across instances → instance-scoped store.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Expose as transaction-tool actions (`capture_schema_snapshot`, `check_schema_drift`) next cycle.
- Index/trigger/constraint-level diffing once column-level proves out in use.

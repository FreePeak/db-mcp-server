# Cycle 63 — Cross-Database Schema Compare

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #2 (cycle 59): schema drift check only compares a database
  against its own saved snapshot — no way to diff two databases (staging vs
  production, primary vs replica) to verify a migration landed everywhere.
- Design: structural-only compare via the existing introspection seams;
  no row data read, works across engines.

## Shipped

- `internal/usecase/schema_compare.go`:
  - `collectSchemaSnapshot`: table → column → lowered type map per dbID.
  - `CompareSchemas(ctx, dbIDA, dbIDB)`: tables only on one side; per
    shared table columns missing on either side (with the present type) and
    type mismatches. Clean "Schemas match: N common table(s)" when equal.
- Schema tool: `format: "compare"` + required `compare_with` database id,
  routed through capability interface `schemaCompareUseCase`.

## Verification

- TDD RED first (undefined CompareSchemas → build fail), then GREEN:
  - `TestCompareSchemas_Diffs`: two SQLite handles via a new multiRepo test
    double — missing column, extra column, extra table all reported.
  - `TestCompareSchemas_Match`: identical schemas produce clean match.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Include indexes/constraints in the compare (currently columns only).
- Oracle session view behind the cloud harness.
- Optional --export-dir for server-side file dumps with path sandboxing.
- Post-merge: verify npm v1.12.0 + docker tags published.

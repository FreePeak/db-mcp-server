# Cycle 147 — Required-INSERT-Columns Audit (schema action=required_columns)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- NOT NULL columns without a DEFAULT are exactly the columns an INSERT
  must supply — the difference between a generated INSERT that runs
  first try and one that fails on constraint violations. DescribeTable
  already returns is_nullable + column_default in its column rows
  (aliased per engine), so the audit needed no new catalog queries.
  Confirmed no existing action covered it.

## Shipped

- `internal/usecase/insert_requirements.go`:
  - `isRequiredColumn(isNullable, hasDefault)` — pure classifier across
    engine encodings (PG "NO", Oracle "N", SQLite pragma notnull=1);
    unknown encodings conservatively not flagged.
  - `InsertRequirements(ctx, dbID)` — walks the table listing via
    GetDatabaseInfo + DescribeTable; renders per-table required-column
    lists sorted by table/column; defaultable-only tables omitted;
    fully clean databases state so explicitly with tables scanned.
- Schema tool: new action `required_columns` (per-db and unified) via
  capability interface `insertRequirementsUseCase`; description string
  updated.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestIsRequiredColumn`: all encodings × default presence, unknown
    → false.
  - `TestInsertRequirements_SQLite`: email (NOT NULL, no default)
    flagged; bio/name/note not; PK id not flagged (rowid alias has
    notnull=0).
  - `TestInsertRequirements_Clean`: explicit clean result.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README schema-tool row for action=required_columns.
- Post-merge: verify npm v1.12.0 + docker tags published.

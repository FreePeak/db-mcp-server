# Cycle 66 — INSERT Statement Generation (format=inserts)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #3 (cycle 59): no way to generate backfill/seed DML — agents
  hand-rolled INSERTs from text tables. The export surface from cycle 60
  was the natural home: same pipeline, one more renderer.

## Shipped

- `internal/usecase/query_export.go`: `format=inserts` renders each row as
  `INSERT INTO <table> (cols) VALUES (...);`. Table name extracted from the
  statement (`SELECT ... FROM <table>` shape required; anything else errors
  explicitly). Numeric cells stay unquoted so DML round-trips types;
  strings are '' escaped; NULL stays a literal.
- Query tool routes the new format through the existing capability
  interface; help text updated.

## Verification

- TDD RED first (unsupported-format error), then GREEN:
  - `TestExecuteQueryFormat_Inserts`: exact two-row output including
    embedded quotes and a NULL literal.
  - `TestExecuteQueryFormat_InsertsErrors`: SELECT without FROM rejected.
- All prior export tests unchanged and passing.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=inserts.
- Post-merge: verify npm v1.12.0 + docker tags published.

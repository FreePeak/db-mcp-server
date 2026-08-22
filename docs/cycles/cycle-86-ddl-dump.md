# Cycle 86 — DDL Dump (schema format=ddl)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Reconstructing a schema for migration authoring meant re-deriving DDL
  from describe metadata. SQLite's catalog stores the original CREATE
  text verbatim — one call dumps it.

## Shipped

- `internal/usecase/ddl.go`: `DumpDDL(ctx, dbID)` — sqlite_master
  statements in creation order, internal `sqlite_%` objects excluded,
  trailing semicolons normalized; non-SQLite engines get an explicit
  "not yet supported" error rather than wrong output.
- Schema tool: `format: "ddl"` routed via capability interface
  `ddlDumpUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDumpDDL`: CREATE TABLE users, CREATE INDEX idx_email, and
    CREATE VIEW adults all dumped verbatim.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=ddl.
- Post-merge: verify npm v1.12.0 + docker tags published.

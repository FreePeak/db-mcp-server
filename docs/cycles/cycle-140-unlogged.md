# Cycle 140 — Unlogged-Table Audit (performance action=unlogged_tables)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- CREATE UNLOGGED TABLE skips WAL — faster bulk loads, but the table is
  truncated on crash recovery and never replicated to standbys. A "temp"
  staging table that quietly became load-bearing is a data-loss incident
  waiting for the first crash. Only pg_class.relpersistence reveals
  them. Confirmed absent.

## Shipped

- `internal/usecase/unlogged.go`:
  - `unloggedTableQuery` — user-schema tables (relkind r/p, non-system
    schemas) with relpersistence='u'; Postgres-only.
  - `ListUnloggedTables(ctx, dbID)` — renders each WAL-skipping table
    with its crash/replication consequences and the
    ALTER TABLE … SET LOGGED fix; clean result stated explicitly.
    Other engines error "not available".
- Performance tool: new action `unlogged_tables` (both per-db and
  unified constructors) served via capability interface
  `unloggedTableUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestUnloggedCatalog`: hits relpersistence + 'u' + non-system
    schema scoping; mysql/sqlite "".
  - `TestListUnloggedTables_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=unlogged_tables.
- Post-merge: verify npm v1.12.0 + docker tags published.

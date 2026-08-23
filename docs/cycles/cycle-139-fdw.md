# Cycle 139 — Foreign-Table (FDW) Discovery (performance action=foreign_tables)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- postgres_fdw links make remote tables look local — a query against
  one silently crosses to another database, which changes both the
  performance story (network latency, remote load) and the security
  surface (data leaves this instance). Only the SQL/MED catalogs reveal
  which tables are not what they seem. Confirmed absent.

## Shipped

- `internal/usecase/fdw.go`:
  - `foreignTableQuery` — pg_foreign_table joined to pg_class /
    pg_namespace / pg_foreign_server, ordered by server then schema;
    Postgres-only.
  - `ListForeignTables(ctx, dbID)` — renders every FDW server grouped
    with its proxied table names and server options, closing with the
    "queries against these read from the REMOTE system" warning; clean
    state explicit ("every table is stored locally"). Other engines
    error "not available".
- Performance tool: new action `foreign_tables` (both per-db and
  unified constructors) served via capability interface
  `foreignTableUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFDWCatalog`: hits pg_foreign_table + pg_foreign_server +
    relnamespace scoping; mysql/sqlite "".
  - `TestListForeignTables_Unsupported`: explicit error.
  - Self-catch: removed an unused total counter before it could trip
    lint.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=foreign_tables.
- Post-merge: verify npm v1.12.0 + docker tags published.

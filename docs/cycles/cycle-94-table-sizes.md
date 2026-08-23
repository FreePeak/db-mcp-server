# Cycle 94 — Table Size Report (schema format=sizes)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- "Which tables are biggest / growing?" needed N describe calls plus
  hand-written per-engine size SQL. One catalog query answers it.

## Shipped

- `internal/usecase/table_sizes.go`: `TableSizes(ctx, dbID)` — Postgres
  (pg_stat_user_tables + pg_total_relation_size) and MySQL
  (information_schema TABLES) report estimate + bytes; engines without
  size catalogs fall back to exact COUNT(*) per table. Sorted heaviest
  first, empty tables included, humanBytes rendering.
- Schema tool: `format: "sizes"` routed via capability interface
  `tableSizeUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestTableSizes`: big table shows its exact 25 rows, empty table is
    listed with 0, heaviest sorts first.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=sizes.
- Post-merge: verify npm v1.12.0 + docker tags published.

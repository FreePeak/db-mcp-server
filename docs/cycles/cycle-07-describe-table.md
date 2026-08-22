# Cycle 07 — Table/Column Schema Depth (`describe` tool)

**Status:** ✅ Shipped · **Artifacts:** PR #85 (this cycle)

## Research Findings
- Domain `SchemaInfo` interface (GetColumns/GetIndexes/GetConstraints) has zero implementations — agents could only list table names via `schema_*`; per-column inspection needed for accurate SQL generation against large schemas was impossible.
- DBHub's `search_objects` covers columns/indexes/procedures; pgEdge advertises schema analysis. Validated gap.

## Shipped
- `DatabaseUseCase.DescribeTable(ctx, dbID, table)`: columns + indexes + best-effort row estimate via first-successful engine catalog query:
  - PostgreSQL/Timescale: `information_schema.columns`, `pg_indexes`, `pg_class.reltuples` estimate
  - MySQL: `SHOW COLUMNS` / `SHOW INDEX`, `information_schema.tables.table_rows`
  - SQLite: `pragma_table_info()` table function, `sqlite_master` indexes, exact COUNT
  - Oracle: `user_tab_columns`, `user_indexes`, `user_tables.num_rows`
- Identifier validation (`^[A-Za-z_][A-Za-z0-9_$]*(\.[...])?$`) blocks catalog-query injection through the table parameter.
- New `describe` tool type registered per-database and unified, rendering compact text output.
- Four test mocks extended for the new UseCaseProvider method.

## Verification
- End-to-end on real in-memory SQLite: 3 columns detected by name, exact row count returned.
- Injection-rejection tests for hostile table parameters.
- go vet / golangci-lint / full suite green.

## Fed Forward
Progressive disclosure for very wide schemas (DBHub pattern) noted as future refinement; constraints (FK/PK) not yet surfaced.

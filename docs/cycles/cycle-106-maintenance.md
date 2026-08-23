# Cycle 106 — Maintenance Suggestions (schema format=maintenance)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Session Notes

- First attempt duplicated RelatedRows + views (both already shipped;
  context compression lost the feature surface) and clobbered two
  tracked files. Reverted cleanly; hard lessons added to LOOP_STATE
  (grep tool_types.go before any RED test; never Write over a file
  without checking git status).

## Shipped

- `internal/usecase/maintenance.go`: `ListMaintenance(ctx, dbID)` —
  Postgres: ≥10% dead-tuple bloat on non-trivial tables suggests
  VACUUM ANALYZE, never-analyzed tables suggest ANALYZE; MySQL:
  >1MB tables with ≥10% free space suggest OPTIMIZE TABLE. Catalog
  reads only; suggested statements rendered for review, never run.
- Schema tool: `format: "maintenance"` via capability interface
  `maintenanceUseCase`.

## Verification

- TDD RED first, then GREEN:
  - `TestListMaintenance_Unsupported`: SQLite errors with "not
    available".
  - `TestMaintenanceCatalog`: pg hits pg_stat_user_tables/n_dead_tup,
    mysql hits DATA_FREE, sqlite returns no catalog.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=maintenance.
- Post-merge: verify npm v1.12.0 + docker tags published.

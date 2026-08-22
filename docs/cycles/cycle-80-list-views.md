# Cycle 80 — View Listing (schema format=views)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Schema introspection covered tables/columns/indexes/FKs only; derived
  tables were invisible, so agents couldn't discover or reuse views.
  Closes the last survey gap item ("no view listing").

## Shipped

- `internal/usecase/views.go`: `ListViews(ctx, dbID)` — engine-catalog
  query per family (pg_views excluding system schemas, information_schema
  for MySQL/SQLite sqlite_master), rendering name + whitespace-collapsed
  definition truncated at 300 chars; unsupported engines error clearly.
- Schema tool: `format: "views"` routed via capability interface
  `viewListingUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestListViews`: active_users listed with its WHERE clause.
- Two compile fixes en route (unused nameCol/defCol returns).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=views.
- Post-merge: verify npm v1.12.0 + docker tags published.

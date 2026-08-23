# Cycle 137 — Invalid-Index Audit (performance action=invalid_indexes)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A crashed CREATE INDEX CONCURRENTLY leaves an INVALID index behind —
  the planner ignores it (so it helps no read) but every write still
  maintains it (so it costs like an index). Nothing on the tool surface
  distinguished valid from invalid. Confirmed absent.

## Shipped

- `internal/usecase/invalid_indexes.go`:
  `ListInvalidIndexes(ctx, dbID)` — reads pg_index/pg_class/pg_namespace
  for user-schema indexes with indisvalid=false, rendering each with
  schema, table, and index name plus the REINDEX INDEX CONCURRENTLY /
  DROP INDEX fix. Clean result stated explicitly. Postgres-only; other
  engines error "not available".
- Performance tool: new action `invalid_indexes` (both per-db and
  unified constructors) served via capability interface
  `invalidIndexUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestInvalidIndexCatalog`: hits indisvalid + pg_index + non-system
    schema scoping; mysql/sqlite "".
  - `TestListInvalidIndexes_Unsupported`: explicit error.
  - Self-catch: first implementation referenced a nonexistent
    quoteQualified helper — build failed in GREEN phase; replaced with
    inline quoteIdent composition.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=invalid_indexes.
- Post-merge: verify npm v1.12.0 + docker tags published.

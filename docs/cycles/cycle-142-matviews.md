# Cycle 142 — Unpopulated Materialized-View Audit (performance action=unpopulated_matviews)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A matview created WITH NO DATA (or whose initial populate failed)
  errors on every query until someone runs REFRESH MATERIALIZED VIEW —
  and it looks identical to a working view from the outside. Only
  pg_matviews.ispopulated reveals which views are shells. Confirmed
  absent.

## Shipped

- `internal/usecase/matviews.go`:
  - `unpopulatedMatviewQuery` — pg_matviews WHERE NOT ispopulated;
    Postgres-only.
  - `ListUnpopulatedMatviews(ctx, dbID)` — renders each shell matview
    with the REFRESH MATERIALIZED VIEW fix; clean result stated
    explicitly ("every matview is queryable"). Other engines error
    "not available".
- Performance tool: new action `unpopulated_matviews` (both per-db and
  unified constructors) served via capability interface
  `matviewUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestMatviewCatalog`: hits pg_matviews + ispopulated;
    mysql/sqlite "".
  - `TestListUnpopulatedMatviews_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=unpopulated_matviews.
- Post-merge: verify npm v1.12.0 + docker tags published.

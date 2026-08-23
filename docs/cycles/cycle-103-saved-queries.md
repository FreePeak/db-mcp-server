# Cycle 103 — Saved Query Bookmarks (query save_query / saved_queries / run_saved_query)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Agents re-pasting the same exploratory SELECT across turns had no
  bookmark primitive. Named per-database replay closes it.

## Shipped

- `internal/usecase/saved_query.go`: `SaveQuery` (1–128 char names,
  100-per-db cap, overwrite allowed), `ListSavedQueries` (sorted, SQL
  preview truncated at 120 chars), `RunSavedQuery` (routes through
  ExecuteQuery so auto-limit and masking apply; database-scoped — a
  bookmark never crosses databases).
- Query tool: `save_query`, `saved_queries: true`, `run_saved_query`
  params routed via capability interface `savedQueryUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestSavedQueries`: save → list shows name+SQL → run returns rows;
    overwrite works; unknown name errors "no saved query"; cross-db run
    rejected.
- Constructor init missed on first wiring (nil map panic caught by the
  test before any commit).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for saved queries.
- Post-merge: verify npm v1.12.0 + docker tags published.

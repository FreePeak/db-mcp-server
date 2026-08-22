# Cycle 77 — Query Pagination (page / page_size)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Paging large result sets meant hand-written LIMIT/OFFSET arithmetic
  plus a separate COUNT call. One paged call returning data + total lets
  an agent decide "more pages?" without extra round-trips.

## Shipped

- `internal/usecase/query_page.go`: `ExecuteQueryPage(ctx, dbID, query,
  params, page, pageSize)` — COUNT over a subquery for the total, then
  the same subquery with LIMIT/OFFSET (1-based page, default 50 rows).
  Read-only enforcement and SELECT/WITH admission shared with count_only;
  output header states page, rows/page, and total.
- Query tool: `page` + `page_size` number params routed via capability
  interface `pagedQueryUseCase`, ahead of count_only.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestExecuteQueryPage`: 22 matching rows, page 2 × 10 returns ids
    11-20; degenerate values (page=0, size=-5) don't error.
  - Removed a nonsense placeholder assertion left in the test draft.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for pagination.
- Post-merge: verify npm v1.12.0 + docker tags published.

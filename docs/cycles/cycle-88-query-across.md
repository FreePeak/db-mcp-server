# Cycle 88 — Cross-Database Fan-Out (query databases=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Spot-checking staging against prod meant N query calls and manual
  comparison. One fan-out call renders per-database sections side by
  side; no competitor MCP offers cross-database execution.

## Shipped

- `internal/usecase/query_across.go`: `ExecuteQueryAcross(ctx, query,
  dbIDs)` — SELECT/WITH admission shared with the read modes; each
  database renders as `=== [id] ===` with its result or its error;
  one bad target never fails the batch.
- Query tool: `databases` comma-separated param routed via capability
  interface `acrossQueryUseCase` (only when more than one id given).

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestExecuteQueryAcross`: both sections render with their own data;
    DELETE rejected up front; an unknown database reports failure in its
    section while the healthy one still returns rows.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for databases=.
- Post-merge: verify npm v1.12.0 + docker tags published.

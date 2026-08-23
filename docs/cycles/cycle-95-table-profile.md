# Cycle 95 — Table Profiling (describe profile=true)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Selectivity/nullability questions ("is status low-cardinality?",
  "how many NULL ages?") meant hand-written aggregate queries per
  column. No competitor MCP profiles tables.

## Shipped

- `internal/usecase/profile.go`: `ProfileTable(ctx, dbID, table)` —
  per column (sorted): row count, NULL count, distinct count, and min/max
  range when non-null; unreadable columns degrade to a note. Identifiers
  validated before interpolation.
- Describe tool: `profile: true` param routed via capability interface
  `tableProfileUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestProfileTable`: rows: 3; status shows distinct: 2 / nulls: 1;
    age present with its range.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for profile=true.
- Post-merge: verify npm v1.12.0 + docker tags published.

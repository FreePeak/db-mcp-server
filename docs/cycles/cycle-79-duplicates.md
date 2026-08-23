# Cycle 79 — Duplicate Detection (duplicates_column)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Data-cleaning passes start with "which values repeat?" — agents had to
  hand-write GROUP BY/HAVING per column. One bounded call answers it.

## Shipped

- `internal/usecase/duplicates.go`: `FindDuplicates(ctx, dbID, table,
  column)` — GROUP BY the column with HAVING COUNT(*) > 1, ordered by
  frequency, capped at 20 groups; renders counts plus one example PK per
  group (PK discovered from constraints, `rowid` fallback for SQLite
  tables without one); clean report when unique.
- Describe tool: `duplicates_column` param routed via capability
  interface `duplicateDetectionUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFindDuplicates`: a@x.io flagged with 3 occurrences and example;
    unique column reports clean; unknown table errors.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for duplicates_column.
- Post-merge: verify npm v1.12.0 + docker tags published.

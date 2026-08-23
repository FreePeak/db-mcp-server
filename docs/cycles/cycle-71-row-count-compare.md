# Cycle 71 — Cross-Database Row-Count Compare

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Schema compare (63-65) proves structure matches; it says nothing about
  whether the data landed. Per-table row counts are the cheap first check
  for "did the seed/migration take?" before any row-level diffing.

## Shipped

- `internal/usecase/data_compare.go`:
  - `countRows`: quoted, portable SELECT COUNT(*).
  - `CompareTableCounts(ctx, dbIDA, dbIDB)`: reuses collectSchemaSnapshot
    for the shared table list; renders `table: a vs b (+delta)` per shared
    table and flags one-sided tables; unreadable tables degrade to a note
    instead of failing the whole compare.
- Schema tool: new format `compare_data_counts` + required
  `compare_with`, routed via capability interface `dataCompareUseCase`.

## Verification

- TDD RED first (undefined symbol → build fail), then GREEN:
  - `TestCompareTableCounts`: users 10 vs 7 reported with "+3"; logs 0/0.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for compare_data_counts.
- Post-merge: verify npm v1.12.0 + docker tags published.

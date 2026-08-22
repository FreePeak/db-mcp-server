# Cycle 72 — Sampled Row-Level Diff

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Row counts (71) prove volume; they don't catch a mutated row. A sampled
  diff of one table — rows present on only one side within the first N by
  first shared column — catches seed/config drift cheaply. Documented as
  sampled, not exhaustive.

## Shipped

- `internal/usecase/data_compare.go`: `CompareTableSamples(ctx, dbA, dbB,
  table, limit)` — shared-column SELECT ordered by the first shared
  column LIMIT n on both sides; tuple-set difference rendered as
  only-in-A / only-in-B lines; clean "Sampled data matches" when equal.
- Schema tool: format `compare_samples` (+ required `compare_with`,
  `table`; optional `limit`), routed via extended `dataCompareUseCase`;
  `table` param added to both schema creators.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestCompareTableSamples`: added (3,c), changed (2,b→changed), and
    (4,d) all reported; self-compare reports a clean match.
  - One syntax slip during implementation (malformed map assignment)
    caught by build immediately.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for compare_samples.
- Post-merge: verify npm v1.12.0 + docker tags published.

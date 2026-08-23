# Cycle 91 — Value-Search Ranking

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- SearchValues reported hits in table-catalog order; an agent scanning
  for "which table holds this UUID" wants the densest match first.

## Shipped

- `internal/usecase/value_search.go`: hits sort by match count
  descending, then table/column name for stable output.

## Verification

- TDD RED first, then GREEN:
  - `TestSearchValues_RankedByHits`: a five-hit table outranks a one-hit
    table regardless of catalog order; existing search tests unchanged.
- One test compile fix en route (fmt import).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Post-merge: verify npm v1.12.0 + docker tags published.

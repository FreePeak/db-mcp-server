# Cycle 89 — View Drift in Schema Compare

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- CompareSchemas diffed tables/columns/indexes/constraints but ignored
  views — silent view drift between environments is a classic breakage.

## Shipped

- `internal/usecase/schema_compare.go`: schemaSnapshot gains a views set;
  collectSchemaSnapshot reuses the cycle-80 `viewsQuery` catalog per
  engine; CompareSchemas reports `view "x": only in <db>` for either
  side. Unsupported engines simply contribute no views.

## Verification

- TDD RED first, then GREEN. Real scan bug found en route:
  - The view collector scanned only the first column of a two-column
    catalog row (`sql: expected 2 destination arguments`) — caught by the
    RED test refusing to pass and confirmed with an isolated probe.
- All three existing TestCompareSchemas cases still pass.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README note that compare includes views.
- Post-merge: verify npm v1.12.0 + docker tags published.

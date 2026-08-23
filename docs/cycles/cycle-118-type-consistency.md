# Cycle 118 — Column-Type Consistency Audit (schema format=type_consistency)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The same column name appearing with divergent types across tables
  (customer_id INTEGER here, TEXT there) signals a bad migration,
  broken FK intent, or copy-paste drift — joins on such columns coerce
  or fail at runtime. Static over the DescribeTable metadata already
  collected. Confirmed absent.

## Shipped

- `internal/usecase/type_consistency.go`:
  `FindTypeInconsistencies(ctx, dbID)` — groups column occurrences by
  name across user tables; flags names in ≥2 tables with >1 distinct
  lowercased type, listing each table's declared type and the join
  consequence. Explicit states: single table ("nothing to cross-
  check"), all consistent ("Type-consistent: …").
- Schema tool: `format: "type_consistency"` via capability interface
  `typeConsistencyUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFindTypeInconsistencies`: customer_id INTEGER vs TEXT drift
    flagged naming shipments; consistent ref_id columns silent;
    single-table database renders "No column appears".
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=type_consistency.
- Post-merge: verify npm v1.12.0 + docker tags published.

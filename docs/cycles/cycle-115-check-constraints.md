# Cycle 115 — CHECK-Constraint Listing (schema format=checks)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- CHECK clauses encode business rules — "status IN ('active','closed')",
  "amount >= 0" — that an agent needs to write valid INSERTs or explain
  a failed one. The describe constraint catalog only surfaces PK/FK;
  nothing read check_constraints. Confirmed absent.

## Shipped

- `internal/usecase/checks.go`: `ListCheckConstraints(ctx, dbID)` —
  Postgres joins table_constraints × check_constraints scoped by
  schema; MySQL reads `information_schema.CHECK_CONSTRAINTS` scoped to
  DATABASE(). Clauses grouped by sorted table; explicit empty state
  ("business rules live in application code"); SQLite errors "not
  available".
- Schema tool: `format: "checks"` via capability interface
  `checkConstraintUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestChecksCatalog`: pg hits check_constraints/check_clause;
    mysql hits CHECK_CONSTRAINTS; sqlite none.
  - `TestListCheckConstraints_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=checks.
- Post-merge: verify npm v1.12.0 + docker tags published.

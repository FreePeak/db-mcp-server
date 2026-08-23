# Cycle 122 — FK Referential-Action Audit (schema format=fk_rules)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- An agent deleting parent rows cannot see whether ON DELETE CASCADE
  will silently destroy children or NO ACTION will block the delete —
  DescribeTable carries edge endpoints but not the referential actions.
  Confirmed absent; engine catalogs expose them directly.

## Shipped

- `internal/usecase/fk_rules.go`: `ListFKRules(ctx, dbID)` — reads
  delete_rule/update_rule for every FK edge from
  information_schema (PG: referential_constraints +
  constraint_column_usage keyed on current_schema(); MySQL:
  REFERENTIAL_CONSTRAINTS join KEY_COLUMN_USAGE scoped to DATABASE()).
  CASCADE edges render "deleting a parent row silently deletes all
  matching children", SET NULL renders its nulling behavior, others
  render "delete blocks while children exist". SQLite errors "not
  available".
- Schema tool: `format: "fk_rules"` via capability interface
  `fkRulesUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFKRulesCatalog`: pg hits referential_constraints + delete_rule
    + update_rule; mysql hits the uppercase equivalents; sqlite none.
  - `TestListFKRules_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=fk_rules.
- Post-merge: verify npm v1.12.0 + docker tags published.

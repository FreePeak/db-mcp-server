# Cycle 65 — Constraints in Schema Compare

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward from cycle 64: indexes catch most drift, but a demoted
  primary key or a dropped foreign key is exactly the kind of silent
  migration damage an agent must see. Verified the DescribeTable
  constraint shape first: `{constraint_type, column_name,
  referenced_table, referenced_column}`.

## Shipped

- `internal/usecase/schema_compare.go`: per-table constraint fingerprints
  — `PRIMARY KEY(id)`, `FOREIGN KEY(pid)->p.id` — collected from
  `desc["constraints"]` and diffed as set membership (one-sided
  constraints reported with owning side; identical sets silent).

## Verification

- TDD RED first (missing PRIMARY KEY/FOREIGN KEY in output), then GREEN:
  - `TestCompareSchemas_Constraints`: A has PKs + FK, B has neither — both
    constraint kinds reported; identical PK sets on a fresh pair produce no
    constraint lines.
- All prior compare tests unchanged and passing.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Oracle session view behind the cloud harness.
- Post-merge: verify npm v1.12.0 + docker tags published.

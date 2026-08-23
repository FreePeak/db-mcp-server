# Cycle 113 — Missing-FK-Index Detection (schema format=fk_indexes)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Every unindexed foreign-key child column turns parent DELETE/UPDATE
  into a full child-table scan (the engine checks referential integrity
  row by row). `suggest_indexes` needs a workload; this audit is static
  and catches the problem before any slow query runs. Reuses the cycle-
  111 `indexColumns` definition parser. Confirmed absent.

## Shipped

- `internal/usecase/fk_indexes.go`:
  - `FindMissingFKIndexes(ctx, dbID)` — walks every user table's FK
    edges (`collectFKEdges`) against each table's leading index columns
    (`tableLeadingIndexColumns`); flags unindexed edges sorted by
    table/column with candidate `CREATE INDEX idx_<child>_<col>` DDL.
  - Explicit states: no FKs at all ("nothing to audit"), all covered
    ("No missing foreign-key indexes: N edge(s)"), verify-before-create
    note on findings.
- Schema tool: `format: "fk_indexes"` via capability interface
  `fkIndexUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFindMissingFKIndexes`: coupon_id flagged with DDL; user_id
    (covered by idx_orders_user) silent; fully-covered database renders
    "No missing"; FK-less database renders "No foreign".
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=fk_indexes.
- Post-merge: verify npm v1.12.0 + docker tags published.

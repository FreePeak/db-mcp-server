# Cycle 144 — AUTO_INCREMENT Headroom Audit (performance action=auto_increment_headroom)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- An auto-increment column that reaches its type ceiling breaks every
  insert with a cryptic "out of range" error. INT ids on high-write
  tables are the classic case; the fix is a pre-planned BIGINT
  migration, not an emergency at 3am. PG sequences already had their
  audit (`sequences.go`); the MySQL/MariaDB side was confirmed absent.

## Shipped

- `internal/usecase/auto_increment.go`:
  - `autoIncrementQuery` — information_schema.COLUMNS joined to TABLES
    for EXTRA LIKE '%auto_increment%' with next values, ordered by
    consumption DESC; mysql/mariadb only.
  - `aiCeiling(colType)` — pure per-type ceiling map including
    unsigned variants (int→2^31-1 … bigint unsigned→2^64-1); unknown
    types → 0 → skipped.
  - `aiRiskLine` — AT CEILING (≥100%), WARNING (≥90% of ceiling),
    comfortable renders nothing so the report stays actionable.
  - `AuditAutoIncrement(ctx, dbID)` — renders at-risk counters with
    counts audited; clean results stated explicitly. Other engines
    error "not available".
- Performance tool: new action `auto_increment_headroom` (both per-db
  and unified constructors) served via capability interface
  `autoIncrementUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestAutoIncrementQuery`: catalog shape + engine gating.
  - `TestAICeiling`: all type ceilings incl. unsigned + unknown→0.
  - `TestAIRiskLine`: escalation boundaries proven.
  - `TestAuditAutoIncrement_Unsupported`: explicit error.
- Self-catch: removed an unused aiUsage struct before lint could flag
  it; two test-value bugs found and fixed (2147000000 is *below* the
  int ceiling so correctly lands in WARNING; 1800000000 is ~84%, below
  the ≥90% warning line) — code semantics unchanged, tests now use
  genuinely over/near-ceiling values with comments explaining why.
- Dropped a draft `usage ratio` struct approach for pure functions on
  scanned rows — simpler and directly testable.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=auto_increment_headroom.
- Post-merge: verify npm v1.12.0 + docker tags published.

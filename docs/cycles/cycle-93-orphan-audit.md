# Cycle 93 — FK Integrity Audit (schema format=orphans)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Agents could resolve FK relationships (53) but not ask whether data
  actually honors them. Orphaned child rows from disabled constraints,
  legacy imports, or partial deletes are silent corruption no competitor
  MCP surfaces.

## Shipped

- `internal/usecase/orphans.go`: `AuditOrphans(ctx, dbID)` — walks every
  FOREIGN KEY edge via the describe catalogs, LEFT JOIN-counts children
  with no parent match per edge, reports violations strongest-first;
  clean databases get an explicit "no violations, N edges checked"
  report; unreadable tables are counted and noted, never fatal.
- Schema tool: `format: "orphans"` routed via capability interface
  `orphanAuditUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestAuditOrphans`: orders.user_id -> users.id edge reported with
    its 1 orphan; a clean database reports "no violations". First run
    exposed wording drift in the clean-report string — fixed.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=orphans.
- Post-merge: verify npm v1.12.0 + docker tags published.

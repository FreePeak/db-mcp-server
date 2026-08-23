# Cycle 81 — Atomic Script Execution (execute script=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Migrations meant N separate execute calls with no atomicity — a failure
  mid-sequence left partial state. One transactional script call closes
  the biggest remaining survey gap.

## Shipped

- `internal/usecase/script.go`:
  - `splitScript`: semicolon split honoring single/double-quoted strings;
    empties dropped.
  - `ExecuteScript(ctx, dbID, script)`: read-only rejected up front; all
    statements in one transaction via `Begin(&domain.TxOptions{})`; first
    error rolls back everything and names the offending statement by
    position (rollback failure surfaces loudly); success reports
    per-statement row counts.
- Execute tool: `script` param routed via capability interface
  `scriptExecutionUseCase`, ahead of the required single-statement path.

## Verification

- TDD RED first (build fail), then GREEN. Real nil-pointer bug found:
  - Test helper `sqliteDB.Begin` dereferenced a nil `*TxOptions`; fixed
    both sides — helper guards nil, usecase passes non-nil options.
  - `TestExecuteScript`: both inserts commit together with "2 statement(s)
    executed"; PK violation on statement 2 rolls back statement 1's insert
    (verified by re-query) and names "statement 2".
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for script execution.
- Post-merge: verify npm v1.12.0 + docker tags published.

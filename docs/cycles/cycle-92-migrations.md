# Cycle 92 — Versioned Migration Runner (execute migrate_dir=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #3 closed: migrations meant hand-rolled execute calls with
  no versioning or applied-tracking. Competitor MCP servers leave this
  entirely to the agent. Flyway-style per-migration atomicity composes
  on top of cycle 81's script splitting.

## Shipped

- `internal/usecase/migrations.go`: `RunMigrations(ctx, dbID, dir)` —
  `.sql` files sorted lexicographically; `_mcp_migrations` table tracks
  applied names so re-runs are noops; **each migration is its own
  transaction** (first failure stops the run, keeps earlier migrations
  committed and recorded, never records the failing file); read-only
  databases rejected up front.
- Execute tool: `migrate_dir` param routed via capability interface
  `migrationRunnerUseCase`.

## Verification

- TDD RED first, then GREEN. The RED test caught a real design flaw:
  - First implementation used one transaction for the whole run — a bad
    migration 002 discarded committed migration 001. Refactored to
    per-migration transactions (industry standard) before GREEN.
  - `TestRunMigrations`: 2 files applied once in order; re-run is "No
    pending"; tracking table holds exactly 2 rows.
  - `TestRunMigrations_FailureAtomic`: failure names `002_bad.sql`,
    001 stays applied AND recorded.
- Routing case added to `TestCapabilityRouting`.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for migrate_dir.
- Post-merge: verify npm v1.12.0 + docker tags published.

## Session Note

- Shell cwd silently reset from the worktree to the main checkout during
  an off-loop detour; two cycle-92 files briefly landed on `main` (removed).
  All commands now pin the worktree explicitly. Hackathon history verified
  intact at 69685d3 before continuing.

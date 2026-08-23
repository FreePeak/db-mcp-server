# Cycle 133 — Orphaned Two-Phase Transactions (performance action=prepared_xacts)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A forgotten PREPARE TRANSACTION holds its locks forever and pins the
  vacuum horizon — invisible to list_sessions (no statement is running)
  and to long_transactions (the session may be gone entirely). Only
  pg_prepared_xacts itself reveals them. Confirmed absent.

## Shipped

- `internal/usecase/prepared_xacts.go`:
  `ListPreparedTransactions(ctx, dbID)` — reads pg_prepared_xacts
  ordered by prepare time with gid, database, owner, and age; each line
  carries the ROLLBACK PREPARED / COMMIT PREPARED decision. Empty state
  explicit ("no prepared transactions in flight"). Postgres-only; other
  engines error "not available".
- Performance tool: new action `prepared_xacts` (both per-db and unified
  constructors) served via capability interface `preparedXactUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestPreparedXactCatalog`: hits pg_prepared_xacts + now() -
    prepared; mysql/sqlite "".
  - `TestListPreparedTransactions_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=prepared_xacts.
- Post-merge: verify npm v1.12.0 + docker tags published.

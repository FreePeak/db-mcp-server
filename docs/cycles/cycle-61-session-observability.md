# Cycle 61 — Session Observability + Query Cancel

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #5 (cycle 59): an agent debugging "the database feels slow"
  had no pg_stat_activity/processlist access except raw SQL through query_*,
  which read-only configs may block, and no way to cancel a runaway query.

## Shipped

- `internal/usecase/session_ops.go`:
  - `sessionsQuery(dbType)`: Postgres `pg_stat_activity` (non-idle, ordered
    by query_start) and MySQL/MariaDB `information_schema.processlist`
    catalog SELECTs; "" for engines without a server session model.
  - `cancelQueryStmt(dbType, id)`: `pg_cancel_backend(n)` / `KILL QUERY n`
    — cancel, not terminate; unsupported engines rejected.
  - `ListActiveSessions(ctx, dbID)` renders the live catalog (capped at 100
    rows); `CancelQuery(ctx, dbID, sessionID)` executes the cancel and tells
    the agent to verify via list_sessions. Clean "not supported" errors on
    SQLite/Oracle rather than silent failure.
- Performance tool: new actions `list_sessions` + `cancel_query`
  (`session_id` number required), routed through a capability interface so
  existing mocks/providers stay valid.

## Verification

- TDD RED first (undefined symbols → build fail), then GREEN:
  - `TestSessionsQuery`: engine catalog SQL shape + unsupported engines "".
  - `TestCancelQueryStmt`: exact cancel statements + sqlite rejection.
  - `TestListActiveSessions_SQLiteUnsupported`: explicit not-supported
    error on both methods.
- Two build errors caught mid-GREEN (missing logger import; domain.Database
  write method is Exec not Execute) — fixed immediately.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Lock-wait view joining pg_locks/pg_blocking_pids (Postgres) and
  metadata_locks (MySQL).
- Cross-DB schema compare.
- Optional --export-dir for server-side file dumps with path sandboxing.
- README rows for the new performance actions.

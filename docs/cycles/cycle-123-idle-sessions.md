# Cycle 123 — Idle-Session Listing (performance action=idle_sessions)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- When connection_saturation warns at 80%+, the agent cannot see who
  holds the slots: list_sessions deliberately filters out PG
  `state = 'idle'` sessions to show activity. The complement — the
  connections doing nothing while eating the client ceiling (pool
  leaks, forgotten shells, stuck app instances) — was invisible.
  Confirmed absent.

## Shipped

- `internal/usecase/idle_sessions.go`: `ListIdleSessions(ctx, dbID)` —
  Postgres reads pg_stat_activity WHERE state='idle' oldest-idle first
  (pid, user, application_name, client_addr, idle duration); MySQL
  reads processlist WHERE command='Sleep' ordered by idle seconds.
  Empty state renders "No idle sessions."; ≥10 idle sessions appends a
  pool-misconfiguration/leak hint. SQLite errors "not available".
- Performance tool: new action `idle_sessions` (both per-db and unified
  constructors) served via capability interface
  `idleSessionsUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestIdleSessionsCatalog`: pg hits `state = 'idle'` +
    pg_stat_activity; mysql hits Sleep + processlist; sqlite none.
  - `TestListIdleSessions_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=idle_sessions.
- Post-merge: verify npm v1.12.0 + docker tags published.

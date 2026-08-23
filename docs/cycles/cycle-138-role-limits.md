# Cycle 138 — Role Connection-Limit Audit (performance action=role_connection_limits)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A login role that reaches its CONNECTION LIMIT rejects every new
  session with a confusing error while other roles log in fine.
  list_sessions shows sessions; only pg_roles' rolconnlimit compared to
  live per-role usage reveals who is about to start rejecting.
  Confirmed absent.

## Shipped

- `internal/usecase/role_limits.go`:
  - `roleLimitQuery` — pg_roles (login roles with finite limits) LEFT
    JOIN live per-role session counts from pg_stat_activity;
    Postgres-only.
  - `roleLimitLine(role, limit, inUse)` — pure classifier: AT LIMIT →
    "rejecting new connections NOW"; ≥80% → WARNING; comfortable roles
    render "" so the report stays actionable.
  - `ListRoleConnectionLimits(ctx, dbID)` — renders capped roles at
    risk; explicit clean states for no-risk and no-capped-roles.
- Performance tool: new action `role_connection_limits` (both per-db
  and unified constructors) served via capability interface
  `roleLimitUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestRoleLimitCatalog`: hits pg_roles + rolconnlimit +
    pg_stat_activity + rolcanlogin; mysql/sqlite "".
  - `TestRoleLimitVerdict`: at-limit / near-limit / comfortable
    escalation proven.
  - `TestListRoleConnectionLimits_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=role_connection_limits.
- Post-merge: verify npm v1.12.0 + docker tags published.

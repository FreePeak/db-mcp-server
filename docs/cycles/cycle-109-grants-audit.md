# Cycle 109 — Grants Audit (schema format=grants)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- "Why can't this app read orders?" and least-privilege reviews had no
  tool support — privilege catalogs were never queried. Cycle 109 adds
  the audit, same engine-gated pattern as maintenance/sequences.

## Shipped

- `internal/usecase/grants.go`: `ListGrants(ctx, dbID)` — Postgres
  reads `information_schema.role_table_grants` scoped to
  `current_schema()`; MySQL reads `information_schema.TABLE_PRIVILEGES`
  scoped to `DATABASE()`. Renders privileges grouped by grantee with
  sorted tables and merged privilege lists. Explicit empty state for
  owner-only databases; SQLite errors "not available".
- Schema tool: `format: "grants"` via capability interface
  `grantsUseCase`.

## Verification

- TDD RED first, then GREEN:
  - `TestGrantsCatalog`: pg hits role_table_grants; mysql hits
    TABLE_PRIVILEGES + DATABASE(); sqlite returns no catalog.
  - `TestListGrants_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=grants.
- Post-merge: verify npm v1.12.0 + docker tags published.

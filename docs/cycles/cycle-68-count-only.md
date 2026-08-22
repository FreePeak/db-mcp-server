# Cycle 68 — count_only Row-Count Preview

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fresh pick after all survey gaps closed: agents had to fetch rows (or
  hand-roll COUNT SQL) just to learn how big a SELECT is before deciding
  whether to pull it. Mirrors the dry_run philosophy for reads: price the
  operation first.

## Shipped

- `internal/usecase/query_count.go`:
  - `IsSelectStatement`: leading-verb check (SELECT/WITH only — what the
    wrap supports), reusing the guard's literal-stripping token helpers.
  - `CountQueryRows(ctx, dbID, query, params)`: wraps as
    `SELECT COUNT(*) AS row_count FROM (<query>) AS count_subquery`
    (universal SQL, Oracle-safe), read-only enforcement intact, bypasses
    max_rows since exactly one row returns.
- Query tool `count_only: true` param routed through a new capability
  interface (`rowCountPreviewUseCase`) ahead of the export branch.

## Verification

- TDD RED first (undefined symbol → build fail; also fixed missing fmt
  import and call signature in tests), then GREEN:
  - `TestCountQueryRows`: WHERE-filtered count correct; DELETE rejected.
  - `TestQueryTool_CountOnlyRouting`: count path runs only with the flag;
    plain queries keep the row path.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for count_only.
- Post-merge: verify npm v1.12.0 + docker tags published.

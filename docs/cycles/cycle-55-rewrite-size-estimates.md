# Cycle 55 — Engine-Aware Rewrite Size Estimates

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward thread: `analyzeAlter` warned "column type changes can force a
  full table rewrite ... on large tables" statically — the advisory never
  knew how large the table actually was, so an operator got identical
  wording for a 10-row and a 100M-row table.
- Existing seams made this cheap: `rowEstimateQuery` (cycle from
  describe_table) already knows per-engine estimate SQL (pg_class.reltuples,
  information_schema.tables.table_rows), and `queryScalar` already executes
  best-effort scalar introspection.

## Shipped

- `internal/usecase/ddl_safety.go`:
  - `alterTypeTargets`: extracts deduplicated, schema-stripped table names
    from ALTER statements carrying TYPE/MODIFY/CHANGE (Postgres + MySQL
    dialects); non-rewrite ALTERs yield nothing.
  - `enrichWithRewriteSizes`: for each target, appends a concrete advisory —
    `Table "items" holds ~42 rows (engine estimate); the column type change
    rewrites all of them and can take locks`. Best-effort: any
    introspection failure leaves the report untouched.
  - `ExecuteStatementDryRun` now consumes its context and enriches the
    report before returning.

## Verification

- TDD RED first (`alterTypeTargets` undefined → build fail), then GREEN.
- `TestAlterTypeTargets`: Postgres/MySQL dialects, ADD/DROP COLUMN excluded,
  schema qualification stripped, batch statements partitioned correctly.
- `TestExecuteStatementDryRun_RewriteSizeNote`: in-memory SQLite with 42
  rows; note names the table with ~42; ADD COLUMN gets no size note.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Surface rewrite-size notes in post-execution warnings too (the
  AnalyzeStatementRisk call at database_usecase.go:477 path).
- Content-PII match threshold tuning if masking proves noisy.
- Post-merge: verify npm v1.12.0 + docker tags publish.

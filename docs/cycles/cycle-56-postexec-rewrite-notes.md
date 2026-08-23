# Cycle 56 — Rewrite-Size Notes in Post-Execution Advisories

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Cycle 55 shipped engine-aware rewrite size estimates only on the dry-run
  path; the post-execution advisory in `ExecuteStatement` still rendered the
  static classification, so an actual type change reported "large tables"
  without the live row count it just rewrote.
- The inline advisory block was also untestable — no seam.

## Shipped

- `internal/usecase/database_usecase.go`: extracted
  `postExecutionRiskNotice(ctx, dbID, statement)` — renders the risk notice,
  now enriched via `enrichWithRewriteSizes` before rendering. Returns ""
  below the warn threshold. `ExecuteStatement` calls it non-blockingly as
  before.

## Verification

- TDD RED first (`postExecutionRiskNotice` undefined → build fail), then
  GREEN: `TestPostExecutionRiskNotice_RewriteSize` proves a type-change
  notice against in-memory SQLite names the table with ~7 rows.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Content-PII match threshold tuning if masking proves noisy.
- Post-merge: verify npm v1.12.0 + docker tags publish.

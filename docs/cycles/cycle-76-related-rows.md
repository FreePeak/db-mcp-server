# Cycle 76 — FK Traversal (RelatedRows)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- describe shows FK metadata but agents still hand-write joins per
  relationship to see what a row relates to. One traversal call closes
  that loop; no competitor MCP offers it.

## Shipped

- `internal/usecase/related_rows.go`: `RelatedRows(ctx, dbID, table,
  keyValue)` — resolves the row by the table's single-column PK, then:
  outgoing FKs looked up via a per-FK scalar fetch of this row's FK value
  (parent rows rendered), incoming references found by scanning every
  other table's constraints for FKs targeting this table (child rows
  rendered, capped at 5 per lookup). No-PK tables error explicitly.
- Describe tool: `related_key` param switches to traversal mode via
  capability interface `relatedRowsUseCase`.

## Verification

- TDD RED first (build fail). Design bug caught during GREEN: the first
  implementation used the caller's key value directly against parent
  tables — wrong when the caller passes the child's PK. Rewritten to
  resolve the row first and use its own FK values for parents.
- Two compile fixes en route (missing domain import, unused sort).
- `TestRelatedRows`: books(10) → parent author Ada resolved; authors(1) →
  child book Notes listed; unknown table errors.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for related_key.
- Post-merge: verify npm v1.12.0 + docker tags published.

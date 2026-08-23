# Cycle 131 — Deprecated-Charset Audit (performance action=charset_audit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- utf8mb3 columns are deprecated on MySQL 8 and a migration landmine —
  no supplementary-plane characters (emoji, CJK ext), tighter
  index-length limits, and every modern default expects utf8mb4.
  Invisible from the tool surface until a migration or index build
  fails. Confirmed absent.

## Shipped

- `internal/usecase/charsets.go`: `AuditCharsets(ctx, dbID)` — reads
  information_schema.COLUMNS scoped to DATABASE() for columns on
  utf8mb3/utf8 (both spellings MySQL has used), rendering table.column
  with charset and collation, a count summary, and the conversion hint
  (ALTER TABLE … CONVERT TO CHARACTER SET utf8mb4 with a unique-index
  byte-width caveat). A clean result is stated explicitly ("all text
  columns are on utf8mb4"). MySQL-only; other engines error "not
  available".
- Performance tool: new action `charset_audit` (both per-db and unified
  constructors) served via capability interface `charsetUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestCharsetCatalog`: mysql hits information_schema.COLUMNS +
    utf8mb3 + DATABASE() scoping; sqlite/postgres "".
  - `TestAuditCharsets_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=charset_audit.
- Post-merge: verify npm v1.12.0 + docker tags published.

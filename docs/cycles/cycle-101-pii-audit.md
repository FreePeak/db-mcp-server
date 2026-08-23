# Cycle 101 — Combined PII Audit (schema format=pii_audit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Overview (100) names PII suspects by column name; the content scan
  (57) catches innocently-named columns. Agents had to run both and
  merge by hand — one call now returns both signals deduped.

## Shipped

- `internal/usecase/pii_audit.go`: `AuditPII(ctx, dbID, sampleRows)` —
  merges FindSensitiveColumns + ScanContentPII per table.column; columns
  flagged by both appear once with both signals ("name suggests X;
  content matches Y"); clean databases get an explicit no-findings line.
- Schema tool: `format: "pii_audit"` (optional `sample_rows`, default 50)
  routed via capability interface `piiAuditUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestPIIAudit`: users.email (name+content) appears exactly once;
    users.notes caught by content only; clean db reports no findings.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for pii_audit.
- Post-merge: verify npm v1.12.0 + docker tags published.

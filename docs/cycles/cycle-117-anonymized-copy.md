# Cycle 117 — Anonymized Cross-Database Copy (mask_pii on copy_table)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- CopyTable passes values through verbatim — seeding staging from prod
  copies real customer emails/phones/cards into a lower-trust
  environment. The shared masking rules (maskPIIInText: emails, phones,
  cards, SSNs, IPs) already existed for query results; nothing applied
  them during a copy. Confirmed absent.

## Shipped

- `internal/usecase/copy_table.go` refactored into `copyTableWith` —
  the shared read-transform-insert pipeline with a per-cell
  `valueTransform` hook. `CopyTable` is now a pass-through wrapper;
  new `CopyTableMasked` applies `maskPIIInText(value, column)` to
  string and []byte cells and appends an explicit "anonymized" note to
  its report. No behavior change for existing copies.
- Transaction tool: new optional `mask_pii` boolean param (both per-db
  and unified constructors); when set with copy_table, served via new
  capability interface `tableCopyMaskedUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestCopyTableMasked`: jane@corp.io and +1-555-234-5678 masked in
    the destination; non-PII "Jane Doe" arrives intact; report says
    "Copied 1 row … anonymized".
- Existing copy tests still green after the refactor (pass-through
  transform = identical behavior).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for mask_pii on copy_table.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 119 — Exact-Duplicate Index Detection (redundant_indexes extension)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `coversPrefix` requires the narrow index's column list to be
  strictly shorter than the wide one, so two indexes with identical
  column lists both survived detection — pure write-tax on every
  INSERT/UPDATE with zero read benefit. Confirmed absent.

## Shipped

- `internal/usecase/redundant_indexes.go` extended:
  - `sameColumns` helper (element-wise equality of normalized column lists).
  - Inner scan now also flags exact duplicates, reported exactly once
    per pair (`j > i`) with the later-created index as the drop
    candidate; rendered as "exact duplicate of" vs the prefix case's
    "covered by".
  - Clean-state message updated to name duplicates too.
- No tool-surface change: same schema format `redundant_indexes`.

## Verification

- TDD RED first, then GREEN:
  - `TestRedundantIndexes_IdenticalDuplicates`: idx_email_a/idx_email_b
    pair reported once as duplicate; count of flagged lines == 1.
  - Existing prefix-coverage and unique-exemption tests stay green.
- Lint caught a real bug during verify: my first refactor used
  `switch`, where `break` exits the switch rather than the inner loop
  (staticcheck SA4011) — reverted to `if` chains preserving the
  original break semantics. Re-verified clean.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row wording for redundant_indexes mentions identical duplicates.
- Post-merge: verify npm v1.12.0 + docker tags published.

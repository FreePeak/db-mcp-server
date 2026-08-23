# Cycle 108 — Sequence Exhaustion Audit (schema format=sequences)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Integer key sequences silently fail at their ceiling — a classic
  "inserts randomly failing" incident with no visible cause. Nothing
  in the tool surface read pg_sequences. Cycle 108 adds the audit.

## Shipped

- `internal/usecase/sequences.go`: `ListSequences(ctx, dbID)` — reads
  pg_sequences, flags sequences at ≥80% of max_value with usage % and
  the remediation hint; fresh/zero sequences skipped; clean state is
  explicit ("No sequences near exhaustion across N tracked"). Engine-
  gated: SQLite errors with "not available".
- Helpers: `sequenceCatalog` (per-engine SELECT), `sequenceExhausted`
  (≥80% threshold), `toFloat` (driver-value coercion).
- Schema tool: `format: "sequences"` via capability interface
  `sequenceUseCase`.

## Verification

- TDD RED first, then GREEN:
  - `TestSequenceCatalog`: pg hits pg_sequences/max_value; sqlite none.
  - `TestSequenceRatio`: threshold math incl. boundary and exhausted.
  - `TestListSequences_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=sequences.
- Post-merge: verify npm v1.12.0 + docker tags published.

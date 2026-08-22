# Cycle 64 — Indexes in Schema Compare

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward from cycle 63: column-only compare misses the most common
  migration drift — an index added in staging but never shipped, or a
  UNIQUE constraint downgraded to a plain index.

## Shipped

- `internal/usecase/schema_compare.go`: `schemaSnapshot` now carries
  per-table index fingerprints (`index_name` → whitespace-normalized,
  lowercased `definition`). Compare reports indexes present on only one
  side and same-name definition differences (catches UNIQUE→plain
  downgrades); cosmetic formatting differences do not read as drift.

## Verification

- TDD RED first (build fail), then GREEN. Bugs caught mid-cycle:
  - `return nil` on a struct snapshot type (two compile fixes),
  - nil inner map when a table's first index is recorded.
- `TestCompareSchemas_Indexes`: UNIQUE vs plain idx_email flagged with the
  word "unique"; identical indexes absent from the report. Column tests
  unchanged and passing.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Constraints (PK/FK/unique) in the compare via desc["constraints"].
- Oracle session view behind the cloud harness.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 110 — Primary-Key Diff Across Databases (schema format=key_diff)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Copy/seed verification stopped at row counts (`VerifyCopy`,
  `compare_data_counts`): identical counts can still hide different
  rows. "Which orders exist in prod but not staging?" needed a keyed
  set difference — absent from the surface.

## Shipped

- `internal/usecase/key_diff.go`: `DiffKeys(ctx, dbA, dbB, table)` —
  locates the PK via the constraint catalog, loads both key sets,
  renders shared count plus per-side only-in counts with up to 20
  example keys each. In-sync case renders explicit parity. Guards:
  plain identifier + non-empty table, distinct databases, single-
  column PK required.
- Schema tool: `format: "key_diff"` (requires `compare_with` +
  `table`, mirroring compare_samples) via capability interface
  `keyDiffUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestDiffKeys`: A={1,2,3} vs B={2,3,4} reports keys 1 and 4 on
    the right sides plus the shared count; empty table errors.
- Shared-count math bug caught by the test (double-subtraction of
  symmetric difference) before wiring.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=key_diff.
- Post-merge: verify npm v1.12.0 + docker tags published.

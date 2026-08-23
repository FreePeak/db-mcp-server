# Cycle 99 — Post-Copy Verification (execute verify_copy=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- CopyTable (96) moves rows blind; nothing proved the copy landed.
  Reconciliation closes the copy workflow: copy → verify → act.

## Session Note

- Research initially proposed query cancellation — CancelQuery already
  shipped in session_ops.go from an earlier cycle. Duplicate discarded
  before any wiring; cycle pivoted to verification instead. Lesson:
  grep usecase/ for the capability *before* writing RED tests.

## Shipped

- `internal/usecase/verify_copy.go`: `VerifyCopy(ctx, srcDB, dstDB,
  table)` — validated COUNT(*) on both sides, match confirmed with the
  count, mismatch reported with both counts and delta; missing table
  fails clearly; same-db rejected.
- Execute tool: `verify_copy` + `from_db` params routed via capability
  interface `copyVerifyUseCase`.

## Verification

- TDD RED first, then GREEN:
  - `TestVerifyCopy`: 3-vs-2 reports MISMATCH with both counts; after
    completing the copy it reports match; ghost table fails.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for verify_copy.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 128 — Growth-Rate Projection in Size Baselines (baseline_compare upgrade)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Size baselines stored per-table counts with no capture timestamp, so
  `format: "baseline_compare"` could say "+400" but not how fast — the
  actual capacity-planning question ("+80/day means months of headroom;
  +8000/day means hours"). Confirmed absent.

## Shipped

- `internal/usecase/size_baseline.go`:
  - `sizeBaseline` gains `capturedAt time.Time`, set on capture.
  - `growthRate(delta, elapsed)` — pure helper: positive deltas over
    multi-day windows render ` (+N/day)`; sub-day windows and
    shrinkage stay unprojected rather than inventing noise.
  - `baselineHeader(dbID, tableCount, elapsed)` — report header now
    states baseline age ("captured N day(s) ago") so stale baselines
    are obvious.
- README documents `baseline_capture`/`baseline_compare` for the first
  time.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestGrowthRate`: projection only for positive multi-day deltas;
    shrinkage/zero/sub-day return "".
  - `TestBaselineHeader`: names db, age, table count.
  - One self-caught signature mismatch (test passed duration where impl
    took day-count) fixed by aligning the helper to take the duration.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Post-merge: verify npm v1.12.0 + docker tags published.

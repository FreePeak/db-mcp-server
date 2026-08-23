# Cycle 104 — Size Baselines (schema format=baseline_capture / baseline_compare)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Table sizes (94) answer "what's biggest now"; growth ("is this table
  ballooning?") needed two manual snapshots plus mental subtraction.

## Shipped

- `internal/usecase/size_baseline.go`: `CaptureSizeBaseline(ctx, dbID)`
  stores per-table COUNT(*) as the database's single comparison point
  (re-capture overwrites); `CompareSizeBaseline` renders per-table
  deltas (+N/−N with before→after), "new since baseline" tables, and an
  explicit no-changes state. Unreadable tables drop from both sides.
- Schema tool: `format: "baseline_capture"` / `"baseline_compare"`
  routed via capability interface `sizeBaselineUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestSizeBaseline`: explicit empty pre-baseline state; +3 delta
    after growth; new table flagged; unchanged tables silent-but-listed.
- Unused-import build error caught before wiring.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for baseline formats.
- Post-merge: verify npm v1.12.0 + docker tags published.

# Cycle 58 — Per-Category Hit Counts in ContentPIIFinding

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward from cycle 57: the 5% noise floor is a fixed policy; operators
  tuning it (or auditing why a column was flagged) had no visibility into
  raw match counts — only category names.

## Shipped

- `internal/usecase/content_pii.go`: `ContentPIIFinding.Hits
  map[string]int` (`hits,omitempty` JSON) populated with per-category match
  counts alongside the existing `categories` list.

## Verification

- TDD RED first (`Hits` field undefined → build fail), then GREEN:
  `TestScanContentPII_ReportsHitCounts` proves email hits=10 on a 10-row
  table.
- All prior content-PII tests pass unchanged (additive field).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Post-merge: verify npm v1.12.0 + docker tags publish.
- Fresh research needed next cycle: all seeded fed-forward threads are now
  closed; pick from competitive scan.

# Cycle 57 — Content-PII Match Threshold (Noise Floor)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward thread: `ScanContentPII` flagged a column on a single matching
  sample. The shared `phoneRe` matches any 10+ digit run, so one order or
  tracking number in a large sample flagged the whole column as phone-PII —
  the exact noise scenario the thread anticipated.
- Constraint from existing tests: tiny tables must keep single-hit
  detection (1 match in 3 rows is strong signal). So the threshold has to
  scale with sample size, not impose an absolute minimum.

## Shipped

- `internal/usecase/content_pii.go`:
  - `contentThresholdMet(hits, scanned)`: a category reports only when
    hits*20 >= scanned (>=5% of samples, minimum one hit).
  - Categories below the floor are dropped per column; columns with no
    surviving categories are no longer emitted.

## Verification

- TDD RED first (`contentThresholdMet` undefined → build fail), then GREEN.
- Unit matrix: {1,3}=true, {1,20}=true, {1,21}=false, {5,100}=true,
  {4,100}=false, {50,1000}=true.
- Integration: 100-row table with exactly one phone-shaped junk value in
  `payload` and ten real emails in `contacts` — payload suppressed,
  contacts still reported.
- Test-data bug caught mid-cycle: the first generator produced a matching
  payload in all 100 rows; fixed so only row 0 carries the false positive
  (the failure output made the over-counting obvious).
- Existing single-hit test on a 3-row table still passes unchanged.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- Expose per-category hit counts in ContentPIIFinding for operator tuning.
- Post-merge: verify npm v1.12.0 + docker tags publish.

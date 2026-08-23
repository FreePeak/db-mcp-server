# Cycle 102 — Health Trend (health action=trend)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Health check (46) is a point-in-time snapshot; diagnosing flakiness
  needs trend — is pool exhaustion chronic or one-off? Agents had to
  poll and remember.

## Shipped

- `internal/usecase/health_trend.go`: rolling per-database sample
  history (last 20) with `RecordHealthSample(dbID, open, max)` +
  `HealthTrend(dbID)` rendering open/max with per-step deltas.
  `HealthCheck` records automatically on every fresh check.
- Health tool: `action: "trend"` renders the history instead of a
  fresh check, via capability interface `healthTrendUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestHealthTrend`: explicit empty state; two samples render both
    values with a +4 delta; other databases don't leak in.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=trend.
- Post-merge: verify npm v1.12.0 + docker tags published.

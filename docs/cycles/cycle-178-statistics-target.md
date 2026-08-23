# Cycle 178 — default_statistics_target Audit (performance action=default_statistics_target)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- ANALYZE samples `default_statistics_target` rows per column to
  build the planner's statistics. The default 100 is fine for
  uniform data but underestimates distinct-value counts and skew on
  wide/skewed columns; the resulting row-count misestimates cascade
  into wrong join orders.
- Completes the PG planner-tuning trio alongside random_page_cost
  (cycle 175) and effective_cache_size (cycle 177).
- Fix path names the reload AND the required re-ANALYZE (existing
  tables don't pick up deeper stats until analyzed), plus the
  per-column SET STATISTICS override and the ANALYZE-cost tradeoff.

## Shipped

- `internal/usecase/statistics_target.go`:
  - `statisticsTargetProbe` — current_setting probe; postgres only.
  - `statisticsTargetVerdict` — pure classifier: ≤100 → WARNING with
    fix path incl. ANALYZE step; tuned → "" (audit adds explicit
    clean line); unparseable/non-positive → unreadable note.
  - `AuditStatisticsTarget` — runs the probe, renders verdict or
    explicit healthy line; unsupported engines get an explicit error.
- Performance tool: new action `default_statistics_target` (both
  per-db and unified constructors) served via capability interface
  `statisticsTargetUseCase`.

## Verification

- TDD RED first (build fail), then GREEN after fixing a syntax bug:
  `statisticsTargetVerdict` mixed an if-statement with a switch's
  `default:` clause — converted to a proper expressionless switch.
- Tests: probe shape + engine gating; "500" quiet; "100" escalated
  with ALTER SYSTEM + ANALYZE fix path; ""/"abc" unreadable;
  explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=default_statistics_target.
- Post-merge: verify npm v1.12.0 + docker tags published.

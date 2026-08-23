# Cycle 145 — Slow-Query-Log Observability Audit (performance action=slow_log)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With slow_query_log=OFF (MySQL) or log_min_duration_statement=-1
  (Postgres), the engine records nothing about slow queries — blind
  exactly when "the database feels slow" starts. engine_slow_queries
  reads live digests; the log is the durable record that survives
  restarts and proves what ran at 3am. Confirmed absent.

## Shipped

- `internal/usecase/slow_log.go`:
  - `slowLogQuery` — mysql/mariadb probe (slow_query_log +
    long_query_time) and Postgres probe
    (log_min_duration_statement); sqlite "".
  - `slowLogVerdict` / `pgSlowLogVerdict` — pure classifiers: OFF/-1 →
    WARNING naming the exact SET command; threshold >5s → "high"
    note; else healthy with the effective threshold.
  - `AuditSlowLog(ctx, dbID)` — renders the verdict; other engines
    error "not available".
- Performance tool: new action `slow_log` (both per-db and unified
  constructors) served via capability interface `slowLogUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestSlowLogProbe`: per-engine settings + gating.
  - `TestSlowLogVerdict`: OFF→warning, loose threshold→note,
    healthy→clean.
  - `TestPgSlowLogVerdict`: -1→warning naming setting, healthy clean.
  - `TestAuditSlowLog_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=slow_log.
- Post-merge: verify npm v1.12.0 + docker tags published.

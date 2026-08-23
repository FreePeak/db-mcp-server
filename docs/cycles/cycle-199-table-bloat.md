# Cycle 199 — Postgres Table-Bloat Audit (performance action=bloat)
**Status:** Shipped · **Branch:** hackathon (PR #87)
## Research
- Every UPDATE and DELETE leaves a dead tuple behind; VACUUM reclaims
  them. When dead tuples pile up — autovacuum starved by
  `autovacuum_max_workers` contention or `autovacuum_vacuum_cost_limit`,
  long-running transactions pinning the xmin horizon, update-heavy
  hot tables — scans read garbage pages, indexes bloat alongside the
  heap, and disk grows with data that no longer exists.
- The pile-up is directly visible in `pg_stat_user_tables`
  (`n_live_tup` vs `n_dead_tup`): one catalog SELECT, no extensions
  needed, complements the cycle-190 `autovacuum_disabled` audit (which
  checks the *worker is on*) by checking whether vacuuming is actually
  *keeping up*.
- Ratio thresholds: below ~1000 rows ratios are noise; ≥20% dead
  tuples is a warning (check autovacuum throughput), ≥50% is critical
  (`VACUUM`, or `VACUUM FULL` off-peak for actual space reclaim).
## Shipped
- `internal/usecase/table_bloat.go`:
  - `bloatQuery` — postgres-only per-table live/dead tuple SELECT,
    worst dead counts first; other engines report unsupported like
    every engine-gated audit.
  - `bloatVerdict` — pure verdict with the noise floor (tables under
    1000 rows total are skipped) and the two ratio tiers; healthy
    tables render "" so the report stays actionable.
  - `renderBloatReport` — sorts flagged tables worst-first by
    dead-tuple ratio and states a clean result explicitly.
  - `CheckTableBloat` — engine gate + scan + report.
- Registry: `{"bloat", CheckTableBloat}` appended to the health-audit
  registry, so it also appears inside `health_audit` combined reports
  (skipped silently on non-postgres engines).
- Performance tool: new action `bloat` (per-db + unified) via the
  capability interface; both action-list description strings updated.
## Verification
- TDD RED first (build failure), GREEN after implementation:
  - `TestBloatQuery` — postgres supported, mysql/sqlite not.
  - `TestBloatVerdict` — noise floor (999-row table never flagged),
    warning tier at exactly 20%, critical tier at ≥50% including
    all-dead, boundary cases.
  - `TestRenderBloatReport_Clean` — clean result states zero flagged
    tables explicitly.
- Full package suites green: `go build ./...`,
  `go test ./internal/usecase/ ./internal/delivery/mcp/`.
## Artifacts
- `internal/usecase/table_bloat.go`, `table_bloat_test.go`
- `internal/usecase/health_audit.go` (registry entry)
- `internal/delivery/mcp/tool_types.go` (action dispatch + lists)

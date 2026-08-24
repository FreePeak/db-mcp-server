# Product Cycle Index

Autonomous build loop for db-mcp-server: each cycle runs
Research → Ideas → Validate → Plan → Implement → Test → Push, then feeds
findings back into the next cycle's research. One document per cycle lives
in this directory; this index is the single entry point.

## How to read these docs

Each cycle doc records: objective, competitive/codebase research findings,
what shipped, verification evidence, and artifacts (PRs, commits, issues).

## Cycle Registry

| Cycle | Theme | Status | Doc |
|-------|-------|--------|-----|
| 01 | Read-only bypass fix + max_rows guardrail | ✅ Shipped (PR #85) | [cycle-01](cycle-01-read-only-bypass-and-maxrows.md) |
| 02 | Real transactions replacing stubs | ✅ Shipped (PR #85) | [cycle-02](cycle-02-real-transactions.md) |
| 03 | Engine-level read-only enforcement | ✅ Shipped (PR #85) | [cycle-03](cycle-03-engine-level-readonly.md) |
| 04 | Unified-tools audit + issue triage (#81/#83) | ✅ Actioned | [cycle-04](cycle-04-unified-audit-issue-triage.md) |
| 05 | explain_<db_id> plan-analysis tool | ✅ Shipped (PR #85) | [cycle-05](cycle-05-explain-tool.md) |
| 06 | CHANGELOG backfill v1.7.0–v1.9.0 | ✅ Shipped (PR #85) | [cycle-06](cycle-06-changelog-backfill.md) |
| 07 | Table/column schema depth (describe tool) | ✅ Shipped (PR #85) | [cycle-07](cycle-07-describe-table.md) |
| 08 | Real performance tool (placeholder removed) | ✅ Shipped (PR #85) | [cycle-08](cycle-08-real-performance-tool.md) |
| 09 | Constraints (PK/FK) surfacing in describe | ✅ Shipped (PR #85) | [cycle-09](cycle-09-describe-constraints.md) |
| 10 | Release flow: merge #85, tag v1.11.0, npm + docker distribution | ✅ Shipped (docker blocked on repo secrets — see #83) | [cycle-10](cycle-10-release-flow.md) |
| 11 | Publish-pipeline integrity: fail-fast guards + disclosure (#86) | ✅ Shipped | [cycle-11](cycle-11-publish-pipeline-integrity.md) |
| 12 | Token-efficiency benchmark: unified ~1.1–1.3k tok flat vs per-db 6.3x at 10 DBs | ✅ Shipped | [cycle-12](cycle-12-token-benchmark.md) |
| 13 | Health check tool: pool pressure + engine stats (DBHub parity) | ✅ Shipped | [cycle-13](cycle-13-health-tool.md) |
| 14 | Engine-level slow queries (pg_stat_statements / MySQL digests) | ✅ Shipped | [cycle-14](cycle-14-engine-slow-queries.md) |
| 15 | FK referenced-table resolution in describe | ✅ Shipped | [cycle-15](cycle-15-fk-references.md) |
| 16 | Mermaid ERD via schema tool (`format=mermaid`) | ✅ Shipped | [cycle-16](cycle-16-mermaid-erd.md) |
| 17 | Index advisor: `suggest_indexes` action with alias + column validation | ✅ Shipped | [cycle-17](cycle-17-index-advisor.md) |
| 18 | Composite index synthesis (equality-first, sort appended) | ✅ Shipped | [cycle-18](cycle-18-composite-indexes.md) |
| 19 | Workload-driven index suggestions (`workload_suggestions`) | ✅ Shipped | [cycle-19](cycle-19-workload-indexes.md) |
| 20 | Statement timeout enforcement (DBHub/PMP Pro time-cap parity) | ✅ Shipped | [cycle-20](cycle-20-query-timeout.md) |
| 21 | Guardrail visibility in health + env timeout override | ✅ Shipped | [cycle-21](cycle-21-guardrail-visibility.md) |
| 22 | Documentation currency: timeout semantics + performance actions in README | ✅ Shipped | main (this cycle) |
| 23 | Execution-weighted workload suggestions with per-statement composites | ✅ Shipped | [cycle-23](cycle-23-traffic-weighting.md) |
| 24 | Live protocol smoke: tracker bypass, health formatter, adapter pass-through fixed | ✅ Shipped | [cycle-24](cycle-24-live-smoke.md) |
| 25 | Statement-path tracking + reusable smoke script (`scripts/smoke.sh`) | ✅ Shipped | [cycle-25](cycle-25-smoke-script.md) |
| 26 | Index health analysis (`index_health`): duplicates + redundant indexes | ✅ Shipped | [cycle-26](cycle-26-index-health.md) |
| 27 | Usage-evidence findings: unused/invalid indexes from engine statistics | ✅ Shipped | [cycle-27](cycle-27-usage-evidence.md) |
| 28 | Table bloat/fragmentation findings + repo hygiene + smoke config committed | ✅ Shipped | [cycle-28](cycle-28-bloat.md) |
| 29 | UNUSED observation window (pg stats_reset) + README currency for index_health | ✅ Shipped | [cycle-29](cycle-29-reset-window.md) |
| 30 | Unified `db_health` action with connection-pressure findings | ✅ Shipped | [cycle-30](cycle-30-db-health.md) |
| 31 | Live-engine validation: PK-noise fix + TestDbHealth_Live against real PostgreSQL | ✅ Shipped | [cycle-31](cycle-31-live-validation.md) |
| 32 | Live MySQL validation: sys.schema_unused_indexes column fix + TestDbHealth_LiveMySQL | ✅ Shipped | [cycle-32](cycle-32-mysql-live-validation.md) |
| 33 | Repeatable live-engine setup script (`scripts/live-db-setup.sh`) | ✅ Shipped | [cycle-33](cycle-33-live-setup-script.md) |
| 34 | Live-engine regression tests wired into CI (seed + port mappings) | ✅ Shipped | [cycle-34](cycle-34-ci-live-tests.md) |
| 35 | Column masking scoping doc — name-based v1 needs no SQL parsing | ✅ Shipped | [cycle-35](cycle-35-masking-scoping.md) |
| 36 | Column masking core: name-based rules wired into ExecuteQuery (Masking Cycle A) | ✅ Shipped | [cycle-36](cycle-36-masking-core.md) |
| 37 | Masking hardening: fail-closed validation, partial strategy, masked-cell counts (Cycle B) | ✅ Shipped | [cycle-37](cycle-37-masking-hardening.md) |
| 38 | Explain output carries index suggestions — closes backlog #9 loop | ✅ Shipped | [cycle-38](cycle-38-explain-advice-wiring.md) |
| 39 | Live-engine masking validation — catches MySQL []byte driver bug in partial strategy | ✅ Shipped | [cycle-39](cycle-39-masking-live-validation.md) |
| 40 | Duration-weighted workload ranking — engine total time beats traffic counts (backlog #8 done) | ✅ Shipped | [cycle-40](cycle-40-duration-weighted-workload.md) |
| 41 | README environment-variables section — closes backlog #10 | ✅ Shipped | [cycle-41](cycle-41-env-vars-doc.md) |
| 42 | Constraint-aware index coverage — PK/UNIQUE columns count as covered (backlog #4 done) | ✅ Shipped | [cycle-42](cycle-42-constraint-aware-coverage.md) |
| 43 | Token-benchmark wire-payload harness + refreshed numbers (backlog #2 hardening) | ✅ Shipped | [cycle-43](cycle-43-token-benchmark-harness.md) |
| 44 | Oracle engine-level read-only via fail-closed privilege audit (backlog #1 done) | ✅ Shipped | [cycle-44](cycle-44-oracle-readonly.md) |
| 45 | Release readiness: CHANGELOG backfill v1.10/v1.11 + stale backlog sweep (#3/#5 done) | ✅ Shipped | [cycle-45](cycle-45-release-readiness.md) |
| 46 | Slow-query index advice + MySQL digest backtick fix (backlog #9 done) | ✅ Shipped | [cycle-46](cycle-46-slow-query-advice.md) |
| 47 | Oracle init scripts connect to TESTDB — fresh-volume reproducibility verified | ✅ Shipped | [cycle-47](cycle-47-oracle-init-fix.md) |
| 48 | CI green: digest-table grant for live advice tests | ✅ Shipped | [cycle-48](cycle-48-ci-grant-fix.md) |
| 49 | Token benchmark re-verification + README drift fix (backlog #2 done) | ✅ Shipped | [cycle-49](cycle-49-token-benchmark-reverify.md) |
| 50 | Verification depth: full suite under -race + whole-repo dead-code sweep clean | ✅ Shipped | [cycle-50](cycle-50-verification-depth.md) |

## Competitive Baseline (researched 2026-08)

| Server | Stars | Key lesson extracted |
|---|---|---|
| DBHub (Bytebase) | ~3.2k | Token-lean surface (2 tools), read-only-by-default, dual-layer guardrails (classifier + engine), max_rows/timeouts |
| MCP Toolbox (Google) | ~15.8k | Tool curation cuts context up to 30x; OIDC identity injection |
| Postgres MCP Pro (CrystalDBA) | ~3.1k | Plan analysis + hypothetical-index tuning; restricted mode = read-only + time caps |
| Bytebase MCP | ~14k | Governance: identity, masking, review, audit |

## Standing Backlog (fed by each cycle)

1. Oracle engine-level read-only — fully done (cycle 44: fail-closed privilege audit + native oracle-free image)
2. Token-efficiency benchmark — fully done (cycle 43 harness + cycle 49 re-verification: unified ~1.25–1.6k tok flat vs per-db ~800/db, 80% saving at 10 DBs)
3. Merge PR #85 → tag release — done (merged 2026-08-22 with green CI; tagged v1.10.0/v1.11.0; CHANGELOG backfilled in cycle 45)
4. Hypothetical-index tuning depth — fully done (cycles 17–18 composites + 19/23/40 workload-driven analysis + 42 constraint-aware coverage)
5. Column masking / governance features (Bytebase differentiator) — fully done (cycles 35 scoping + 36 core + 37 hardening + 39 live validation)
6. FK referenced-table detail — fully done (cycle 15 + database-wide Mermaid relationship graph via performance tool format=mermaid)
7. After #86 secrets land: verify docker tags v1.9.0/v1.10.0/v1.11.0 + npm 1.11.0; consider npm OIDC trusted publishing
8. Workload-driven tuning — fully done (cycles 19, 23, 40: execution weighting → composites → duration-weighted ranking)
9. Wire suggest_indexes into explain/slow-query output — fully done (cycle 38 explain + cycle 46 slow-queries with bounded catalog reads)
10. Guardrail visibility + env timeout override — fully done (cycle 21 + cycle 41 README environment-variables section)
11. Column masking / SELECT-target resolution — needs real SQL parsing or catalog help; scope as multi-cycle effort first

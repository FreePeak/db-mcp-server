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

## Competitive Baseline (researched 2026-08)

| Server | Stars | Key lesson extracted |
|---|---|---|
| DBHub (Bytebase) | ~3.2k | Token-lean surface (2 tools), read-only-by-default, dual-layer guardrails (classifier + engine), max_rows/timeouts |
| MCP Toolbox (Google) | ~15.8k | Tool curation cuts context up to 30x; OIDC identity injection |
| Postgres MCP Pro (CrystalDBA) | ~3.1k | Plan analysis + hypothetical-index tuning; restricted mode = read-only + time caps |
| Bytebase MCP | ~14k | Governance: identity, masking, review, audit |

## Standing Backlog (fed by each cycle)

1. Oracle engine-level read-only enforcement (needs live Oracle container)
2. Token-efficiency benchmark: unified vs per-db vs DBHub claim
3. Merge PR #85 → tag v1.10.0 when CI green
4. Hypothetical-index tuning depth — heuristic advisor shipped (cycles 17–18, incl. composites); constraint-aware coverage and workload-driven analysis remain
5. Column masking / governance features (Bytebase differentiator)
6. FK referenced-table detail — done (cycle 15); multi-table relationship graph remains
7. After #86 secrets land: verify docker tags v1.9.0/v1.10.0/v1.11.0 + npm 1.11.0; consider npm OIDC trusted publishing
8. Workload-driven tuning — shipped (cycles 19, 23: weighted by executions, per-statement composites); duration-weighted ranking is the possible refinement
9. Wire suggest_indexes into explain/slow-query output so plan analysis points at concrete fixes (close the Postgres MCP Pro loop)
10. Guardrail visibility + env timeout override — shipped (cycle 21); remaining: document QUERY_TIMEOUT_SECONDS in README config section
11. Column masking / SELECT-target resolution — needs real SQL parsing or catalog help; scope as multi-cycle effort first

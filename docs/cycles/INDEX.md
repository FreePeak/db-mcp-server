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
| 17 | Index advisor (`suggest_indexes` action, alias-safe) | ✅ Shipped (hackathon branch) | [cycle-17](cycle-17-index-advisor.md) |
| 18 | Cloud-DB test harness: DSN parsing + auto-register CLI (Docker-free) | ✅ Shipped (hackathon branch) | [cycle-18](cycle-18-cloud-test-harness.md) |
| 19 | PII masking at tool boundary (`mask_pii`, opt-in, two-layer) | ✅ Shipped (hackathon branch) | [cycle-19](cycle-19-pii-masking.md) |
| 20 | Advisor v2: composite index suggestions + PK suppression | ✅ Shipped (hackathon branch) | [cycle-20](cycle-20-advisor-v2.md) |
| 21 | Operator-enforced masking (`mask_pii` config, bypass-proof) | ✅ Shipped (hackathon branch) | [cycle-21](cycle-21-enforced-masking.md) |
| 22 | Hardening: Luhn card precision + cloud cold-start retry | ✅ Shipped (hackathon branch) | [cycle-22](cycle-22-hardening.md) |

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
4. Hypothetical-index tuning depth (Postgres MCP Pro differentiator)
5. Column masking / governance features (Bytebase differentiator)
6. FK referenced-table detail — done (cycle 15); multi-table relationship graph remains
7. After #86 secrets land: verify docker tags v1.9.0/v1.10.0/v1.11.0 + npm 1.11.0; consider npm OIDC trusted publishing

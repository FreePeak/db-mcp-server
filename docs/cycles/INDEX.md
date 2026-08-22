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

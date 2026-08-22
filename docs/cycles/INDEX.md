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
| 23 | Release consolidation: CHANGELOG entries + PR to main | ✅ Shipped (hackathon branch) | [cycle-23](cycle-23-release-consolidation.md) |
| 24 | Masking audit log (ring buffer, health-surfaced) | ✅ Shipped (hackathon branch) | [cycle-24](cycle-24-masking-audit.md) |
| 25 | Durable audit sink: `-masking-audit-log` JSONL append trail | ✅ Shipped (hackathon branch) | [cycle-25](cycle-25-audit-sink.md) |
| 26 | Docs surface + schema-contract guard (caught per-db mask_pii drift) | ✅ Shipped (hackathon branch) | [cycle-26](cycle-26-docs-contract.md) |
| 27 | Result verbosity modes (`minimal`/`normal`/`full`, token compression) | ✅ Shipped (hackathon branch) | [cycle-27](cycle-27-verbosity.md) |
| 28 | Per-database default verbosity (config-level, precedence-safe) | ✅ Shipped (hackathon branch) | [cycle-28](cycle-28-db-verbosity.md) |
| 29 | Offline DDL risk analysis (`dry_run`, multi-engine pre-flight) | ✅ Shipped (hackathon branch) | [cycle-29](cycle-29-ddl-safety.md) |
| 30 | Post-execution risk advisories (non-blocking warnings) | ✅ Shipped (hackathon branch) | [cycle-30](cycle-30-risk-advisories.md) |
| 31 | Configurable risk warning threshold (`SetRiskWarnAt`) | ✅ Shipped (hackathon branch) | [cycle-31](cycle-31-risk-threshold.md) |
| 32 | Pre-mutation snapshots + reverse-SQL rollback | ✅ Shipped (hackathon branch) | [cycle-32](cycle-32-snapshots.md) |
| 33 | Agent-facing snapshot actions (`list_snapshots`/`rollback_snapshot`) | ✅ Shipped (hackathon branch) | [cycle-33](cycle-33-snapshot-actions.md) |
| 34 | Snapshot audit context (originating statement) + docs surface | ✅ Shipped (hackathon branch) | [cycle-34](cycle-34-snapshot-context.md) |
| 35 | Schema snapshots & drift detection (baseline + diff) | ✅ Shipped (hackathon branch) | [cycle-35](cycle-35-schema-drift.md) |
| 36 | Agent-facing drift actions (`capture`/`check`/`list`) | ✅ Shipped (hackathon branch) | [cycle-36](cycle-36-drift-actions.md) |
| 37 | Safe-migration workflow recipe + CI verification | ✅ Shipped (hackathon branch) | [cycle-37](cycle-37-migration-recipe.md) |
| 38 | Release prep: package.json 1.12.0 + CHANGELOG cut | ✅ Shipped (hackathon branch) | [cycle-38](cycle-38-release-prep.md) |
| 39 | Sensitive column discovery (`format=sensitive`) | ✅ Shipped (hackathon branch) | [cycle-39](cycle-39-sensitive-columns.md) |
| 40 | Content-based PII detection (sampled, pattern-confirmed) | ✅ Shipped (hackathon branch) | [cycle-40](cycle-40-content-pii.md) |
| 41 | Merged sensitive report (name + content sections in one payload) | ✅ Shipped (hackathon branch) | [cycle-41](cycle-41-merged-report.md) |
| 42 | Operator surface: -risk-warn-at flag + docs coverage | ✅ Shipped (hackathon branch) | [cycle-42](cycle-42-operator-surface.md) |
| 43 | Coverage audit & gap closure (usecase 83.3%) | ✅ Shipped (hackathon branch) | [cycle-43](cycle-43-coverage-audit.md) |
| 44 | Tool-registry integration tests (per-db vs unified verified) | ✅ Shipped (hackathon branch) | [cycle-44](cycle-44-registry-tests.md) |
| 45 | Declarative env defaults for operator flags | ✅ Shipped (hackathon branch) | [cycle-45](cycle-45-env-defaults.md) |
| 46 | Auto-LIMIT injection (server-side bound, Oracle-safe) | ✅ Shipped (hackathon branch) | [cycle-46](cycle-46-auto-limit.md) |
| 47 | Auto-LIMIT documentation (guardrails table + explainer) | ✅ Shipped (hackathon branch) | [cycle-47](cycle-47-auto-limit-docs.md) |
| 48 | Query history ring + `list_query_history` action | ✅ Shipped (hackathon branch) | [cycle-48](cycle-48-query-history.md) |
| 49 | Loop-continuity infrastructure (`LOOP_STATE.md`) | ✅ Shipped (hackathon branch) | — |
| 50 | Durable query-history sink (JSONL, fail-fast config) | ✅ Shipped (hackathon branch) | [cycle-50](cycle-50-history-sink.md) |

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

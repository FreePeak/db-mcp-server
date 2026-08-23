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
| 51 | History sink server wiring (-query-history-log + env) | ✅ Shipped (hackathon branch) | [cycle-51](cycle-51-history-wiring.md) |
| 52 | PR #87 title/body refresh for full scope | ✅ Shipped (hackathon branch) | [cycle-52](cycle-52-pr-refresh.md) |
| 53 | Session report (`docs/SESSION-REPORT.md`) | ✅ Shipped (hackathon branch) | [cycle-53](cycle-53-session-report.md) |
| 54 | Oracle Auto-LIMIT via ROWNUM wrap (closes exclusion gap from 46) | ✅ Shipped (hackathon branch) | [cycle-54](cycle-54-oracle-autolimit.md) |
| 55 | Engine-aware rewrite size estimates in dry-run risk reports | ✅ Shipped (hackathon branch) | [cycle-55](cycle-55-rewrite-size-estimates.md) |
| 56 | Rewrite-size notes in post-execution risk advisories | ✅ Shipped (hackathon branch) | [cycle-56](cycle-56-postexec-rewrite-notes.md) |
| 57 | Content-PII noise floor (>=5% match threshold) | ✅ Shipped (hackathon branch) | [cycle-57](cycle-57-content-pii-threshold.md) |
| 58 | Per-category hit counts in ContentPIIFinding | ✅ Shipped (hackathon branch) | [cycle-58](cycle-58-pii-hit-counts.md) |
| 59 | generate_schema tool (Go structs / TS types from live schema) | ✅ Shipped (hackathon branch) | [cycle-59](cycle-59-generate-schema.md) |
| 60 | Query export formats (CSV RFC4180 / JSON) | ✅ Shipped (hackathon branch) | [cycle-60](cycle-60-query-export.md) |
| 61 | Session observability (list_sessions / cancel_query) | ✅ Shipped (hackathon branch) | [cycle-61](cycle-61-session-observability.md) |
| 62 | Lock-wait view (lock_waits) + README catch-up | ✅ Shipped (hackathon branch) | [cycle-62](cycle-62-lock-waits.md) |
| 63 | Cross-database schema compare (schema format=compare) | ✅ Shipped (hackathon branch) | [cycle-63](cycle-63-schema-compare.md) |
| 64 | Index fingerprints in schema compare | ✅ Shipped (hackathon branch) | [cycle-64](cycle-64-schema-compare-indexes.md) |
| 65 | Constraint fingerprints (PK/FK) in schema compare | ✅ Shipped (hackathon branch) | [cycle-65](cycle-65-schema-compare-constraints.md) |
| 66 | INSERT generation via query format=inserts | ✅ Shipped (hackathon branch) | [cycle-66](cycle-66-insert-generation.md) |
| 67 | PR #87 description refresh (cycles 52-66) | ✅ Shipped (hackathon branch) | [cycle-67](cycle-67-pr-refresh.md) |
| 68 | count_only row-count preview on query tool | ✅ Shipped (hackathon branch) | [cycle-68](cycle-68-count-only.md) |
| 69 | Single-column statistical profile | ✅ Shipped (hackathon branch) | [cycle-69](cycle-69-column-profile.md) |
| 70 | SESSION-REPORT refresh for cycles 52-69 | ✅ Shipped (hackathon branch) | [cycle-70](cycle-70-session-report.md) |
| 71 | Cross-DB row-count compare (compare_data_counts) | ✅ Shipped (hackathon branch) | [cycle-71](cycle-71-row-count-compare.md) |
| 72 | Sampled row-level diff (compare_samples) | ✅ Shipped (hackathon branch) | [cycle-72](cycle-72-sample-diff.md) |
| 73 | Per-query timeout (timeout_ms param) | ✅ Shipped (hackathon branch) | [cycle-73](cycle-73-query-timeout.md) |
| 74 | Continuity doc sync (SESSION-REPORT, LOOP_STATE) | ✅ Shipped (hackathon branch) | [cycle-74](cycle-74-doc-sync.md) |
| 75 | Cross-table value search (filter_tables value=) | ✅ Shipped (hackathon branch) | [cycle-75](cycle-75-value-search.md) |
| 76 | FK traversal for one row (describe related_key=) | ✅ Shipped (hackathon branch) | [cycle-76](cycle-76-related-rows.md) |
| 77 | Query pagination (page/page_size + total) | ✅ Shipped (hackathon branch) | [cycle-77](cycle-77-query-pagination.md) |
| 78 | Random sampling (sample_rows, engine-aware) | ✅ Shipped (hackathon branch) | [cycle-78](cycle-78-random-sample.md) |
| 79 | Duplicate detection (describe duplicates_column=) | ✅ Shipped (hackathon branch) | [cycle-79](cycle-79-duplicates.md) |
| 80 | View listing with definitions (schema format=views) | ✅ Shipped (hackathon branch) | [cycle-80](cycle-80-list-views.md) |
| 81 | Atomic multi-statement scripts (execute script=) | ✅ Shipped (hackathon branch) | [cycle-81](cycle-81-scripts.md) |
| 82 | Trigger listing (schema format=triggers) | ✅ Shipped (hackathon branch) | [cycle-82](cycle-82-triggers.md) |
| 83 | Stored routine listing (schema format=routines) | ✅ Shipped (hackathon branch) | [cycle-83](cycle-83-routines.md) |
| 84 | CSV bulk import (execute csv_data=) | ✅ Shipped (hackathon branch) | [cycle-84](cycle-84-csv-import.md) |
| 85 | Custom type listing (schema format=types) | ✅ Shipped (hackathon branch) | [cycle-85](cycle-85-custom-types.md) |
| 86 | Verbatim DDL dump (schema format=ddl, sqlite) | ✅ Shipped (hackathon branch) | [cycle-86](cycle-86-ddl-dump.md) |
| 87 | Continuity doc sync (second pass) | ✅ Shipped (hackathon branch) | [cycle-87](cycle-87-doc-sync-2.md) |
| 88 | Cross-database fan-out (query databases=) | ✅ Shipped (hackathon branch) | [cycle-88](cycle-88-query-across.md) |
| 89 | View drift in schema compare | ✅ Shipped (hackathon branch) | [cycle-89](cycle-89-view-drift.md) |
| 90 | Delivery routing tests (capability wiring) | ✅ Shipped (hackathon branch) | [cycle-90](cycle-90-routing-tests.md) |
| 91 | Value-search ranking by hit count | ✅ Shipped (hackathon branch) | [cycle-91](cycle-91-ranked-search.md) |
| 92 | Versioned migration runner (execute migrate_dir=) | ✅ Shipped (hackathon branch) | [cycle-92](cycle-92-migrations.md) |
| 93 | FK integrity audit (schema format=orphans) | ✅ Shipped (hackathon branch) | [cycle-93](cycle-93-orphan-audit.md) |
| 94 | Table size report (schema format=sizes) | ✅ Shipped (hackathon branch) | [cycle-94](cycle-94-table-sizes.md) |
| 95 | Column profiling (describe profile=true) | ✅ Shipped (hackathon branch) | [cycle-95](cycle-95-table-profile.md) |
| 96 | Cross-database table copy (execute copy_table=) | ✅ Shipped (hackathon branch) | [cycle-96](cycle-96-copy-table.md) |
| 97 | Unused index detection (query unused_indexes=) | ✅ Shipped (hackathon branch) | [cycle-97](cycle-97-unused-indexes.md) |
| 98 | Long-query triage (query long_queries=N) | ✅ Shipped (hackathon branch) | [cycle-98](cycle-98-long-queries.md) |
| 99 | Post-copy verification (execute verify_copy=) | ✅ Shipped (hackathon branch) | [cycle-99](cycle-99-verify-copy.md) |
| 100 | Database overview snapshot (schema format=overview) | ✅ Shipped (hackathon branch) | [cycle-100](cycle-100-overview.md) |
| 101 | Combined PII audit (schema format=pii_audit) | ✅ Shipped (hackathon branch) | [cycle-101](cycle-101-pii-audit.md) |
| 102 | Health trend history (health action=trend) | ✅ Shipped (hackathon branch) | [cycle-102](cycle-102-health-trend.md) |
| 103 | Saved query bookmarks (query save/run/list) | ✅ Shipped (hackathon branch) | [cycle-103](cycle-103-saved-queries.md) |
| 104 | Size baselines for growth diffs (schema format=baseline_*) | ✅ Shipped (hackathon branch) | [cycle-104](cycle-104-size-baselines.md) |
| 105 | Markdown data dictionary (schema format=dictionary) | ✅ Shipped (hackathon branch) | [cycle-105](cycle-105-data-dictionary.md) |
| 106 | Maintenance suggestions from engine stats (format=maintenance) | ✅ Shipped (hackathon branch) | [cycle-106](cycle-106-maintenance.md) |
| 107 | FK-safe dependency order (format=dependency_order) | ✅ Shipped (hackathon branch) | [cycle-107](cycle-107-dependency-order.md) |
| 108 | Sequence exhaustion audit (format=sequences) | ✅ Shipped (hackathon branch) | [cycle-108](cycle-108-sequences.md) |
| 109 | Grants audit (format=grants) | ✅ Shipped (hackathon branch) | [cycle-109](cycle-109-grants-audit.md) |
| 110 | Primary-key diff across databases (format=key_diff) | ✅ Shipped (hackathon branch) | [cycle-110](cycle-110-key-diff.md) |
| 111 | Redundant-index detection (format=redundant_indexes) | ✅ Shipped (hackathon branch) | [cycle-111](cycle-111-redundant-indexes.md) |
| 112 | Idle-in-transaction audit (action=long_transactions) | ✅ Shipped (hackathon branch) | [cycle-112](cycle-112-long-transactions.md) |
| 113 | Missing-FK-index detection (format=fk_indexes) | ✅ Shipped (hackathon branch) | [cycle-113](cycle-113-fk-indexes.md) |
| 114 | Replication status (action=replication_status) | ✅ Shipped (hackathon branch) | [cycle-114](cycle-114-replication-status.md) |
| 115 | CHECK-constraint listing (format=checks) | ✅ Shipped (hackathon branch) | [cycle-115](cycle-115-check-constraints.md) |
| 116 | No-primary-key audit (format=no_pk) | ✅ Shipped (hackathon branch) | [cycle-116](cycle-116-no-pk.md) |
| 117 | Anonymized cross-database copy (mask_pii) | ✅ Shipped (hackathon branch) | [cycle-117](cycle-117-anonymized-copy.md) |
| 118 | Column-type consistency audit (format=type_consistency) | ✅ Shipped (hackathon branch) | [cycle-118](cycle-118-type-consistency.md) |
| 119 | Exact-duplicate index detection (redundant_indexes ext.) | ✅ Shipped (hackathon branch) | [cycle-119](cycle-119-duplicate-indexes.md) |
| 120 | Connection saturation (action=connection_saturation) | ✅ Shipped (hackathon branch) | [cycle-120](cycle-120-connection-saturation.md) |
| 121 | Timeout-guardrail audit (action=timeout_guardrails) | ✅ Shipped (hackathon branch) | [cycle-121](cycle-121-timeout-guardrails.md) |
| 122 | FK referential-action audit (format=fk_rules) | ✅ Shipped (hackathon branch) | [cycle-122](cycle-122-fk-rules.md) |
| 123 | Idle-session listing (action=idle_sessions) | ✅ Shipped (hackathon branch) | [cycle-123](cycle-123-idle-sessions.md) |
| 124 | Temp-spill detection (action=temp_spills) | ✅ Shipped (hackathon branch) | [cycle-124](cycle-124-temp-spills.md) |
| 125 | Sequential-scan workload audit (action=seq_scan_heavy) | ✅ Shipped (hackathon branch) | [cycle-125](cycle-125-seq-scans.md) |

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

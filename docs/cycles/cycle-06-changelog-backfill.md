# Cycle 06 — CHANGELOG Backfill v1.7.0–v1.9.0

**Status:** ✅ Shipped · **Artifacts:** PR #85, commit `95a4550`

## Research Findings
- CHANGELOG stopped at v1.6.1 (2025-04) while v1.7.0–v1.9.0 shipped major capabilities: SQLite support (#47), lazy loading, configurable log dir, npm distribution (#48), Oracle support (#51), filter_table_names, TimescaleDB tool category + editor context integration (v1.8.0), multi-arch Docker images (#27/#31 in v1.7.0).
- Adopters evaluating against DBHub/MCP Toolbox see a dead changelog — adoption-facing weakness.

## Shipped
- Reconstructed structured release notes from tag-range git history with accurate tag dates (v1.7.0 2025-04-24, v1.8.0 2025-05-09, v1.9.0 2026-04-11), grouped Added/Fixed per release with PR references.

## Method Note
Used `git for-each-ref` for tag dates + `git log --format="%s" <range>` per release; deduplicated merge-commit noise into user-facing entries.

# Cycle 54 — Release v1.12.0

**Status:** Shipped · **Artifacts:** tag `v1.12.0`

## Shipped
- Cut **v1.12.0**: 52 commits since v1.11.0. CHANGELOG Unreleased section promoted and completed with the features that had shipped after it was last touched:
  - hypopg planner validation (`validate_suggestions`), Oracle statement statistics (`v$sqlarea`), JSONL audit trail (`DB_MCP_AUDIT_LOG`), duration-ranked workload suggestions, index advice in explain + slow-queries views, guardrail visibility in db_health, README env-vars reference.
- package.json bumped 1.11.0 → 1.12.0 (npm-publish derives its version from it).
- Tag pushed; CI green on the release commit.

## Distribution status (unchanged blockers)
- **Docker**: fails fast on missing `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets — the cycle-11 guard working as designed; tracked in issue #83.
- **npm**: fails fast on missing `NPM_TOKEN` secret, same pattern.
- Both re-run automatically once secrets land (backlog #7); no code changes needed.
